package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func initDB(dsn string) *gorm.DB {
	dataDir := resolveDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "bank.db")
	log.Printf("Using database at: %s", dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
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
	if err := migrate(sqlDB); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := seedDefaultBanks(db); err != nil {
		log.Printf("seed banks: %v", err)
	}
	// if os.Getenv("SEED_DEV") == "1" {
	// 	if err := seedDevData(sqlDB); err != nil {
	// 		log.Fatalf("seed: %v", err)
	// 	}
	// }
	return db
}

func resolveDataDir() string {
	if dir := os.Getenv("APP_DATA_DIR"); dir != "" {
		return dir
	}

	if path := os.Getenv("SQLITE_PATH"); path != "" {
		return filepath.Dir(path)
	}

	if cwd, err := os.Getwd(); err == nil {
		if fileExists(filepath.Join(cwd, "go.mod")) {
			return filepath.Join(cwd, "data")
		}
	}

	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "bank-consolidation")
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".bank-consolidation")
	}

	if exe, err := os.Executable(); err == nil && exe != "" {
		return filepath.Join(filepath.Dir(exe), "data")
	}

	return "data"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func migrate(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id VARCHAR(64) PRIMARY KEY,
			type VARCHAR(32) NOT NULL,
			name VARCHAR(255) NOT NULL,
			default_account VARCHAR(255) NULL,
			business_rules TEXT NULL,
			tax_rules TEXT NULL,
			budget_ref VARCHAR(255) NULL,
			deleted_at DATETIME NULL
		)`,
		`CREATE TABLE IF NOT EXISTS bank_types (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			code VARCHAR(20) NOT NULL,
			description TEXT NULL,
			format VARCHAR(50) DEFAULT 'GENERIC',
			deleted_at DATETIME NULL,
			UNIQUE(name),
			UNIQUE(code)
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id VARCHAR(64) PRIMARY KEY,
			raw_csv LONGTEXT NOT NULL,
			import_source VARCHAR(255) NULL,
			validation_status VARCHAR(32) NOT NULL,
			import_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME NULL
		)`,
		`CREATE TABLE IF NOT EXISTS transaction_categories (
			transaction_id VARCHAR(64) NOT NULL,
			category_id VARCHAR(64) NOT NULL,
			UNIQUE (transaction_id, category_id)
		)`,
		`CREATE TABLE IF NOT EXISTS invoice_headers (
			id VARCHAR(64) PRIMARY KEY,
			invoice_no VARCHAR(64) NOT NULL,
			invoice_date DATETIME NOT NULL,
			customer_id VARCHAR(64) NOT NULL,
			customer_name VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL,
			total_amount DECIMAL(15,2) NOT NULL,
			total_tax DECIMAL(15,2) NOT NULL,
			company_code VARCHAR(64) NOT NULL,
			deleted_at DATETIME NULL
		)`,
		`CREATE TABLE IF NOT EXISTS invoice_details (
			id VARCHAR(64) PRIMARY KEY,
			header_id VARCHAR(64) NOT NULL,
			product_id VARCHAR(64) NOT NULL,
			description VARCHAR(255) NOT NULL,
			quantity DECIMAL(15,4) NOT NULL,
			unit_price DECIMAL(15,4) NOT NULL,
			amount DECIMAL(15,4) NOT NULL,
			tax_rate DECIMAL(5,2) NOT NULL,
			tax_amount DECIMAL(15,4) NOT NULL,
			deleted_at DATETIME NULL
		)`,
		`CREATE VIEW IF NOT EXISTS v_invoice_summary AS
			SELECT id AS header_id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code
			FROM invoice_headers
			WHERE deleted_at IS NULL`,
		`CREATE VIEW IF NOT EXISTS v_transaction_category_summary AS
			SELECT tc.transaction_id, t.import_source, t.validation_status, tc.category_id, c.type AS category_type, c.name AS category_name
			FROM transaction_categories tc
			JOIN transactions t ON t.id = tc.transaction_id
			JOIN categories c ON c.id = tc.category_id`,
		`CREATE TABLE IF NOT EXISTS bank_entries (
			id VARCHAR(64) PRIMARY KEY,
			transaction_date DATETIME NOT NULL,
			description TEXT NOT NULL,
			branch VARCHAR(32) NOT NULL,
			amount DECIMAL(18,2) NOT NULL,
			amount_type VARCHAR(2) NOT NULL,
			balance DECIMAL(18,2) NOT NULL,
			bank_code VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN',
			is_finalized BOOLEAN DEFAULT FALSE,
			fingerprint VARCHAR(64) NULL,
			deleted_at DATETIME NULL,
			UNIQUE(fingerprint)
		)`,
		`CREATE TABLE IF NOT EXISTS bank_entry_invoices (
			bank_entry_id VARCHAR(64) NOT NULL,
			invoice_header_id VARCHAR(64) NOT NULL,
			matched_amount DECIMAL(18,2) NULL,
			note TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (bank_entry_id, invoice_header_id)
		)`,
	}

	for i, s := range tables {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("table stmt %d: %s: %w", i, s, err)
		}
	}

	// Add columns manually if they don't exist (compatibility for older SQLite)
	if !columnExists(db, "bank_types", "format") {
		log.Println("Adding 'format' column to bank_types table...")
		if _, err := db.Exec("ALTER TABLE bank_types ADD COLUMN format VARCHAR(50) DEFAULT 'GENERIC'"); err != nil {
			log.Printf("Failed to add 'format' column: %v", err)
			// Don't fail hard, just log error, maybe column already exists or other issue
		} else {
			log.Println("Successfully added 'format' column to bank_types table")
		}
	}
	if !columnExists(db, "bank_entries", "bank_code") {
		if _, err := db.Exec("ALTER TABLE bank_entries ADD COLUMN bank_code VARCHAR(16) NOT NULL DEFAULT 'BRI'"); err != nil {
			return fmt.Errorf("add column bank_code: %w", err)
		}
	}
	if !columnExists(db, "bank_entries", "fingerprint") {
		if _, err := db.Exec("ALTER TABLE bank_entries ADD COLUMN fingerprint VARCHAR(64) NULL"); err != nil {
			return fmt.Errorf("add column fingerprint: %w", err)
		}
	}
	if !columnExists(db, "bank_entries", "is_finalized") {
		if _, err := db.Exec("ALTER TABLE bank_entries ADD COLUMN is_finalized BOOLEAN DEFAULT FALSE"); err != nil {
			return fmt.Errorf("add column is_finalized: %w", err)
		}
	}

	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_bank_entries_fingerprint ON bank_entries (fingerprint)`,
		`CREATE INDEX IF NOT EXISTS idx_bank_entries_bankcode_date ON bank_entries (bank_code, transaction_date)`,
		`CREATE INDEX IF NOT EXISTS idx_bank_entries_bankcode_branch_date ON bank_entries (bank_code, branch, transaction_date)`,
		`CREATE INDEX IF NOT EXISTS idx_bank_entries_amount_type ON bank_entries (amount_type)`,
		`CREATE INDEX IF NOT EXISTS idx_bank_entries_branch ON bank_entries (branch)`,
		`CREATE INDEX IF NOT EXISTS idx_bank_entries_deleted_at ON bank_entries (deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_bank_entry_invoices_invoice ON bank_entry_invoices (invoice_header_id)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_headers_invoice_date ON invoice_headers (invoice_date)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_headers_status_date ON invoice_headers (status, invoice_date)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_headers_company_code_date ON invoice_headers (company_code, invoice_date)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_headers_customer_id_date ON invoice_headers (customer_id, invoice_date)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_headers_invoice_no ON invoice_headers (invoice_no)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_details_header_id ON invoice_details (header_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transaction_categories_transaction_id ON transaction_categories (transaction_id)`,
	}

	for i, s := range indexes {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("index stmt %d: %s: %w", i, s, err)
		}
	}

	return nil
}

func columnExists(db *sql.DB, tableName, columnName string) bool {
	var count int
	// Use pragma_table_info to check if column exists
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", tableName, columnName)
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func seedDevData(db *sql.DB) error {
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(1) FROM categories`).Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		_, err := db.Exec(`INSERT INTO categories (id, type, name, default_account, business_rules, tax_rules, budget_ref) VALUES
			('CAT-IN-001','money_in','Penjualan','401','{}','{}','BUD-2025'),
			('CAT-OUT-001','money_out','Pembelian','501','{}','{}','BUD-2025')`)
		if err != nil {
			return err
		}
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM transactions`).Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		_, err := db.Exec(`INSERT INTO transactions (id, raw_csv, import_source, validation_status) VALUES
			('TX-001','id,amount,desc\nTX-001,150000,Sample income','csv','pending'),
			('TX-002','id,amount,desc\nTX-002,80000,Sample expense','csv','pending')`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES
			('TX-001','CAT-IN-001'),
			('TX-002','CAT-OUT-001')`)
		if err != nil {
			return err
		}
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM invoice_headers`).Scan(&cnt); err != nil {
		return err
	}
	if cnt < 100 {
		// Ensure initial seed exists
		if cnt == 0 {
			_, err := db.Exec(`INSERT INTO invoice_headers (id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code)
				VALUES ('INV-H-001','INV-001',CURRENT_TIMESTAMP,'CUST-001','PT Contoh','pending',230000,23000,'COMP-01')`)
			if err != nil {
				return err
			}
			_, err = db.Exec(`INSERT INTO invoice_details (id, header_id, product_id, description, quantity, unit_price, amount, tax_rate, tax_amount) VALUES
				('INV-D-001','INV-H-001','PROD-001','Produk A',2,50000,100000,10,10000),
				('INV-D-002','INV-H-001','PROD-002','Produk B',1,120000,120000,10,12000)`)
			if err != nil {
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

			// Random date in Nov-Dec 2025
			month := time.November
			if rand.Intn(2) == 1 {
				month = time.December
			}
			day := rand.Intn(28) + 1
			invDate := time.Date(2025, month, day, rand.Intn(24), rand.Intn(60), 0, 0, time.UTC)

			cust := customers[rand.Intn(len(customers))]
			status := statuses[rand.Intn(len(statuses))]
			if i%5 != 0 {
				status = "pending" // bias towards pending
			}
			company := "COMP-01"
			if rand.Intn(2) == 1 {
				company = "COMP-02"
			}

			// Generate details first to sum amount
			numDetails := rand.Intn(4) + 1
			var totalAmount, totalTax float64

			tx, err := db.Begin()
			if err != nil {
				return err
			}

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

				_, err = tx.Exec(`INSERT INTO invoice_details (id, header_id, product_id, description, quantity, unit_price, amount, tax_rate, tax_amount) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					dID, invID, prod.ID, prod.Name, qty, price, amt, taxRate, tax)
				if err != nil {
					tx.Rollback()
					return err
				}
			}

			_, err = tx.Exec(`INSERT INTO invoice_headers (id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				invID, invNo, invDate, cust.ID, cust.Name, status, totalAmount, totalTax, company)
			if err != nil {
				tx.Rollback()
				return err
			}
			tx.Commit()
		}
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM bank_entries`).Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		_, err := db.Exec(`INSERT INTO bank_entries (id, transaction_date, description, branch, amount, amount_type, balance, bank_code) VALUES
			('BE-001','2025-11-29 00:00:00','BI-FAST CR TANGGAL :28/11 TRANSFER   DR 002  DAHNIAR   ','0000',34244370.00,'CR',342889691.38,'BRI'),
			('BE-002','2025-11-29 00:00:00','BI-FAST CR TRANSFER   DR 002 HAJAR NURUL A''IN    ','0000',1978000.00,'CR',344867691.38,'BRI'),
			('BE-003','2025-11-29 00:00:00','TRSF E-BANKING CR 2911/FTSCY/WS95271 525150.00  Kaffeine ERIANSYAH, S.PI  ','0000',525150.00,'CR',345392841.38,'BRI'),
			('BE-004','2025-11-29 00:00:00','TRSF E-BANKING CR 2911/FTSCY/WS95271 2961790.00  nota sinar anugrah 27 nov 2025  BUDI SANTOSO ','0000',2961790.00,'CR',348354631.38,'BRI'),
			('BE-005','2025-11-29 00:00:00','SWITCHING CR TRF 3 SRI ASTUTI  002  Web BRILink  ','0998',191000.00,'CR',348545631.38,'BRI')
		`)
		if err != nil {
			return err
		}
	}
	return nil
}
