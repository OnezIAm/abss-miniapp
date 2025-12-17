package main

import (
	"bank-consolidation/models"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func initDB(dsn string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if os.Getenv("SEED_DEV") == "1" {
		if err := seedDevData(db); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}
	return db
}

func migrate(db *gorm.DB) error {
	// AutoMigrate tables using GORM models
	err := db.AutoMigrate(
		&models.Category{},
		&models.Transaction{},
		&models.TransactionCategory{},
		&models.InvoiceHeader{},
		&models.InvoiceDetail{},
		&models.BankEntry{},
		&models.BankEntryInvoice{},
	)
	if err != nil {
		return err
	}

	// Create Views and complex Indexes
	stmts := []string{
		`CREATE OR REPLACE VIEW v_invoice_summary AS
			SELECT id AS header_id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code
			FROM invoice_headers
			WHERE deleted_at IS NULL`,
		`CREATE OR REPLACE VIEW v_transaction_category_summary AS
			SELECT tc.transaction_id, t.import_source, t.validation_status, tc.category_id, c.type AS category_type, c.name AS category_name
			FROM transaction_categories tc
			JOIN transactions t ON t.id = tc.transaction_id
			JOIN categories c ON c.id = tc.category_id`,
		// Indexes (GORM handles most via tags, but ensuring multi-column indexes)
		// Use CREATE INDEX IF NOT EXISTS (MySQL 8.0+) or handle error
	}

	// Helper to execute raw SQL safely (ignoring "already exists" errors if syntax doesn't support IF NOT EXISTS)
	// MySQL 5.7 doesn't support IF NOT EXISTS for indexes in CREATE INDEX? It does usually.
	// But let's try to run them.
	indexStmts := []string{
		`CREATE INDEX idx_bank_entries_bankcode_date ON bank_entries (bank_code, transaction_date)`,
		`CREATE INDEX idx_bank_entries_bankcode_branch_date ON bank_entries (bank_code, branch, transaction_date)`,
		`CREATE INDEX idx_bank_entries_amount_type ON bank_entries (amount_type)`,
		`CREATE INDEX idx_bank_entries_branch ON bank_entries (branch)`,
		`CREATE INDEX idx_invoice_headers_status_date ON invoice_headers (status, invoice_date)`,
		`CREATE INDEX idx_invoice_headers_company_code_date ON invoice_headers (company_code, invoice_date)`,
		`CREATE INDEX idx_invoice_headers_customer_id_date ON invoice_headers (customer_id, invoice_date)`,
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			log.Printf("Migration warning (views): %v", err)
		}
	}

	// For indexes, we can use a check before creating, or just try and ignore specific error.
	// Simpler to just run and ignore "Duplicate key name" error (Error 1061)
	for _, s := range indexStmts {
		if err := db.Exec(s).Error; err != nil {
			// log.Printf("Migration warning (index): %v", err)
			// Silence index errors as they likely exist
		}
	}

	return nil
}

func seedDevData(db *gorm.DB) error {
	var cnt int64
	if err := db.Model(&models.Category{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		cats := []models.Category{
			{ID: "CAT-IN-001", Type: "money_in", Name: "Penjualan", DefaultAccount: "401", BusinessRules: "{}", TaxRules: "{}", BudgetRef: "BUD-2025"},
			{ID: "CAT-OUT-001", Type: "money_out", Name: "Pembelian", DefaultAccount: "501", BusinessRules: "{}", TaxRules: "{}", BudgetRef: "BUD-2025"},
		}
		if err := db.Create(&cats).Error; err != nil {
			return err
		}
	}

	if err := db.Model(&models.Transaction{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		txs := []models.Transaction{
			{ID: "TX-001", RawCSV: "id,amount,desc\nTX-001,150000,Sample income", ImportSource: "csv", ValidationStatus: "pending"},
			{ID: "TX-002", RawCSV: "id,amount,desc\nTX-002,80000,Sample expense", ImportSource: "csv", ValidationStatus: "pending"},
		}
		if err := db.Create(&txs).Error; err != nil {
			return err
		}

		txCats := []models.TransactionCategory{
			{TransactionID: "TX-001", CategoryID: "CAT-IN-001"},
			{TransactionID: "TX-002", CategoryID: "CAT-OUT-001"},
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&txCats).Error; err != nil {
			return err
		}
	}

	if err := db.Model(&models.InvoiceHeader{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt < 100 {
		if cnt == 0 {
			h := models.InvoiceHeader{
				InvoiceHeaderID: "INV-H-001",
				InvoiceNo:       "INV-001",
				InvoiceDate:     time.Now(),
				CustomerID:      "CUST-001",
				CustomerName:    "PT Contoh",
				Status:          "pending",
				TotalAmount:     230000,
				TotalTax:        23000,
				CompanyCode:     "COMP-01",
			}
			if err := db.Create(&h).Error; err != nil {
				return err
			}

			details := []models.InvoiceDetail{
				{InvoiceDetailID: "INV-D-001", InvoiceHeaderID: "INV-H-001", ProductID: "PROD-001", ProductName: "Produk A", Qty: 2, UnitPrice: 50000, Amount: 100000, PpnPercent: 10, Ppn: 10000},
				{InvoiceDetailID: "INV-D-002", InvoiceHeaderID: "INV-H-001", ProductID: "PROD-002", ProductName: "Produk B", Qty: 1, UnitPrice: 120000, Amount: 120000, PpnPercent: 10, Ppn: 12000},
			}
			if err := db.Create(&details).Error; err != nil {
				return err
			}
			cnt++
		}

		// Generate remaining random records
		customers := []struct {
			ID   string
			Name string
		}{
			{"CUST-001", "PT Contoh"}, {"CUST-002", "CV Maju Jaya"}, {"CUST-003", "Toko Abadi"},
			{"CUST-004", "UD Sentosa"}, {"CUST-005", "PT Gemilang"}, {"CUST-006", "Warung Sejahtera"},
			{"CUST-007", "CV Berkah"}, {"CUST-008", "PT Sinar Harapan"}, {"CUST-009", "Toko Makmur"},
			{"CUST-010", "UD Lancar"},
		}
		products := []struct {
			ID   string
			Name string
		}{
			{"PROD-001", "Produk A"}, {"PROD-002", "Produk B"}, {"PROD-003", "Produk C"},
			{"PROD-004", "Produk D"}, {"PROD-005", "Produk E"},
		}
		statuses := []string{"pending", "paid", "overdue", "void"}

		for i := cnt; i < 100; i++ {
			invID := fmt.Sprintf("INV-H-%03d", i+1)
			invNo := fmt.Sprintf("INV/2025/XII/%04d", i+1)

			month := time.November
			if rand.Intn(2) == 1 {
				month = time.December
			}
			day := rand.Intn(28) + 1
			invDate := time.Date(2025, month, day, rand.Intn(24), rand.Intn(60), 0, 0, time.UTC)

			cust := customers[rand.Intn(len(customers))]
			status := statuses[rand.Intn(len(statuses))]
			if i%5 != 0 {
				status = "pending"
			}
			company := "COMP-01"
			if rand.Intn(2) == 1 {
				company = "COMP-02"
			}

			numDetails := rand.Intn(4) + 1
			var totalAmount, totalTax float64

			err := db.Transaction(func(tx *gorm.DB) error {
				for j := 0; j < numDetails; j++ {
					dID := fmt.Sprintf("INV-D-%03d-%d", i+1, j+1)
					prod := products[rand.Intn(len(products))]
					qty := float64(rand.Intn(10) + 1)
					price := float64((rand.Intn(100) + 1) * 10000)
					amt := qty * price
					taxRate := 11.0
					tax := amt * (taxRate / 100)

					totalAmount += amt
					totalTax += tax

					detail := models.InvoiceDetail{
						InvoiceDetailID: dID,
						InvoiceHeaderID: invID,
						ProductID:       prod.ID,
						ProductName:     prod.Name,
						Qty:             qty,
						UnitPrice:       price,
						Amount:          amt,
						PpnPercent:      taxRate,
						Ppn:             tax,
					}
					if err := tx.Create(&detail).Error; err != nil {
						return err
					}
				}

				header := models.InvoiceHeader{
					InvoiceHeaderID: invID,
					InvoiceNo:       invNo,
					InvoiceDate:     invDate,
					CustomerID:      cust.ID,
					CustomerName:    cust.Name,
					Status:          status,
					TotalAmount:     totalAmount,
					TotalTax:        totalTax,
					CompanyCode:     company,
				}
				if err := tx.Create(&header).Error; err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	if err := db.Model(&models.BankEntry{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		be := []models.BankEntry{
			{ID: "BE-001", TransactionDate: parseTime("2025-11-29 00:00:00"), Description: "BI-FAST CR TANGGAL :28/11 TRANSFER   DR 002  DAHNIAR   ", Branch: "0000", Amount: 34244370.00, AmountType: "CR", Balance: 342889691.38, BankCode: "BRI"},
			{ID: "BE-002", TransactionDate: parseTime("2025-11-29 00:00:00"), Description: "BI-FAST CR TRANSFER   DR 002 HAJAR NURUL A'IN    ", Branch: "0000", Amount: 1978000.00, AmountType: "CR", Balance: 344867691.38, BankCode: "BRI"},
			{ID: "BE-003", TransactionDate: parseTime("2025-11-29 00:00:00"), Description: "TRSF E-BANKING CR 2911/FTSCY/WS95271 525150.00  Kaffeine ERIANSYAH, S.PI  ", Branch: "0000", Amount: 525150.00, AmountType: "CR", Balance: 345392841.38, BankCode: "BRI"},
			{ID: "BE-004", TransactionDate: parseTime("2025-11-29 00:00:00"), Description: "TRSF E-BANKING CR 2911/FTSCY/WS95271 2961790.00  nota sinar anugrah 27 nov 2025  BUDI SANTOSO ", Branch: "0000", Amount: 2961790.00, AmountType: "CR", Balance: 348354631.38, BankCode: "BRI"},
			{ID: "BE-005", TransactionDate: parseTime("2025-11-29 00:00:00"), Description: "SWITCHING CR TRF 3 SRI ASTUTI  002  Web BRILink  ", Branch: "0998", Amount: 191000.00, AmountType: "CR", Balance: 348545631.38, BankCode: "BRI"},
		}
		if err := db.Create(&be).Error; err != nil {
			return err
		}
	}
	return nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return t
}
