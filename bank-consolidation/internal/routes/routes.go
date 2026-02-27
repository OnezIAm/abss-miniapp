package routes

import (
	"bank-consolidation/internal/controllers"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(db *gorm.DB) *gin.Engine {
	inv := controllers.InvoiceController{DB: db}
	txc := controllers.TransactionController{DB: db}
	cat := controllers.CategoryController{DB: db}
	be := controllers.BankEntryController{DB: db}
	bt := controllers.BankTypeController{DB: db}

	r := gin.Default()

	// Serve Next.js static files
	r.Static("/_next", "./frontend/out/_next")
	r.Static("/themes", "./frontend/out/themes")
	r.Static("/demo", "./frontend/out/demo")
	r.StaticFile("/favicon.ico", "./frontend/out/favicon.ico")

	// Serve index.html for root
	r.GET("/", func(c *gin.Context) {
		c.File("./frontend/out/index.html")
	})

	// Fallback for other routes (HTML files or 404)
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// API check
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		cleanPath := filepath.Clean(path)
		fullPath := filepath.Join("./frontend/out", cleanPath)

		// Try exact match (e.g. /robots.txt)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			c.File(fullPath)
			return
		}

		// Try .html extension (e.g. /banking -> /banking.html)
		htmlPath := fullPath + ".html"
		if info, err := os.Stat(htmlPath); err == nil && !info.IsDir() {
			c.File(htmlPath)
			return
		}

		// Try index.html in directory
		indexPath := filepath.Join(fullPath, "index.html")
		if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
			c.File(indexPath)
			return
		}

		c.File("./frontend/out/404.html")
	})

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))
	api := r.Group("/api/v1")

	api.POST("/invoices", func(c *gin.Context) { inv.Create(c.Writer, c.Request) })
	api.POST("/invoices/bulk", func(c *gin.Context) { inv.BulkCreate(c.Writer, c.Request) })
	api.POST("/invoices/seed", func(c *gin.Context) { inv.GenerateSample(c.Writer, c.Request) })
	api.GET("/invoices", func(c *gin.Context) { inv.CreateOrList(c.Writer, c.Request) })
	api.GET("/invoices/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/invoices/" + c.Param("id")
		inv.GetByID(c.Writer, c.Request)
	})

	api.POST("/transactions", func(c *gin.Context) { txc.CreateOrList(c.Writer, c.Request) })
	api.GET("/transactions", func(c *gin.Context) { txc.CreateOrList(c.Writer, c.Request) })
	api.POST("/transactions/:id/categories", func(c *gin.Context) {
		c.Request.URL.Path = "/transactions/" + c.Param("id") + "/categories"
		txc.MapCategories(c.Writer, c.Request)
	})

	api.POST("/categories", func(c *gin.Context) { cat.CreateOrList(c.Writer, c.Request) })
	api.GET("/categories", func(c *gin.Context) { cat.CreateOrList(c.Writer, c.Request) })

	// Bank entries CRUD
	api.POST("/bank-entries", func(c *gin.Context) { be.CreateOrList(c.Writer, c.Request) })
	api.POST("/bank-entries/seed", func(c *gin.Context) { be.GenerateSample(c.Writer, c.Request) })

	// Bank types CRUD
	api.POST("/bank-types", func(c *gin.Context) { bt.CreateOrList(c.Writer, c.Request) })
	api.GET("/bank-types", func(c *gin.Context) { bt.CreateOrList(c.Writer, c.Request) })
	api.POST("/bank-entries/bulk", func(c *gin.Context) { be.BulkCreate(c.Writer, c.Request) })
	api.POST("/bank-entries/upload", func(c *gin.Context) { be.UploadCSV(c.Writer, c.Request) })
	api.GET("/bank-entries/export/reconciled", func(c *gin.Context) { be.ExportReconciled(c.Writer, c.Request) })
	api.GET("/bank-entries", func(c *gin.Context) { be.CreateOrList(c.Writer, c.Request) })
	api.GET("/bank-entries/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id")
		be.GetByID(c.Writer, c.Request)
	})
	api.PUT("/bank-entries/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id")
		be.Update(c.Writer, c.Request)
	})
	api.DELETE("/bank-entries/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id")
		be.Delete(c.Writer, c.Request)
	})
	api.POST("/bank-entries/:id/reconcile", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id") + "/reconcile"
		be.Reconcile(c.Writer, c.Request)
	})
	api.GET("/bank-entries/:id/invoices", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id") + "/invoices"
		be.ListAttachedInvoices(c.Writer, c.Request)
	})

	api.GET("/reports/invoices", func(c *gin.Context) {
		var list []map[string]any
		if err := db.Table("v_invoice_summary").Order("invoice_date DESC").Find(&list).Error; err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			return
		}

		// Transform keys to match original response if needed. GORM map scan uses DB column names (snake_case usually).
		// Original response used camelCase.
		// Let's do a manual mapping to ensure API compatibility.
		var responseList []map[string]any
		for _, item := range list {
			responseList = append(responseList, map[string]any{
				"headerId":     item["header_id"],
				"invoiceNo":    item["invoice_no"],
				"invoiceDate":  item["invoice_date"],
				"customerId":   item["customer_id"],
				"customerName": item["customer_name"],
				"status":       item["status"],
				"totalAmount":  item["total_amount"],
				"totalTax":     item["total_tax"],
				"companyCode":  item["company_code"],
			})
		}

		c.Header("Content-Type", "application/json")
		_ = json.NewEncoder(c.Writer).Encode(responseList)
	})

	api.GET("/reports/transactions/categories", func(c *gin.Context) {
		var list []map[string]any
		if err := db.Table("v_transaction_category_summary").Order("transaction_id").Find(&list).Error; err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			return
		}

		var responseList []map[string]any
		for _, item := range list {
			responseList = append(responseList, map[string]any{
				"transactionId":    item["transaction_id"],
				"importSource":     item["import_source"],
				"validationStatus": item["validation_status"],
				"categoryId":       item["category_id"],
				"categoryType":     item["category_type"],
				"categoryName":     item["category_name"],
			})
		}

		c.Header("Content-Type", "application/json")
		_ = json.NewEncoder(c.Writer).Encode(responseList)
	})

	return r
}
