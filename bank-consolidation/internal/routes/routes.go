package routes

import (
	"bank-consolidation/internal/controllers"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Register(db *sql.DB) *gin.Engine {
	inv := controllers.InvoiceController{DB: db}
	txc := controllers.TransactionController{DB: db}
	cat := controllers.CategoryController{DB: db}
	be := controllers.BankEntryController{DB: db}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))
	api := r.Group("/api/v1")

	api.POST("/invoices", func(c *gin.Context) { inv.Create(c.Writer, c.Request) })
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
	api.POST("/bank-entries/bulk", func(c *gin.Context) { be.BulkCreate(c.Writer, c.Request) })
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
		rows, err := db.Query(`SELECT header_id, invoice_no, invoice_date, customer_id, customer_name, status, total_amount, total_tax, company_code FROM v_invoice_summary ORDER BY invoice_date DESC`)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var list []map[string]any
		for rows.Next() {
			var m struct {
				HeaderID, InvoiceNo, InvoiceDate, CustomerID, CustomerName, Status, CompanyCode string
				TotalAmount, TotalTax                                                           float64
			}
			if err := rows.Scan(&m.HeaderID, &m.InvoiceNo, &m.InvoiceDate, &m.CustomerID, &m.CustomerName, &m.Status, &m.TotalAmount, &m.TotalTax, &m.CompanyCode); err != nil {
				http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
				return
			}
			list = append(list, map[string]any{
				"headerId": m.HeaderID, "invoiceNo": m.InvoiceNo, "invoiceDate": m.InvoiceDate,
				"customerId": m.CustomerID, "customerName": m.CustomerName, "status": m.Status,
				"totalAmount": m.TotalAmount, "totalTax": m.TotalTax, "companyCode": m.CompanyCode,
			})
		}
		c.Header("Content-Type", "application/json")
		_ = json.NewEncoder(c.Writer).Encode(list)
	})

	api.GET("/reports/transactions/categories", func(c *gin.Context) {
		rows, err := db.Query(`SELECT transaction_id, import_source, validation_status, category_id, category_type, category_name FROM v_transaction_category_summary ORDER BY transaction_id`)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var list []map[string]string
		for rows.Next() {
			var tid, src, vs, cid, ctype, cname string
			if err := rows.Scan(&tid, &src, &vs, &cid, &ctype, &cname); err != nil {
				http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
				return
			}
			list = append(list, map[string]string{"transactionId": tid, "importSource": src, "validationStatus": vs, "categoryId": cid, "categoryType": ctype, "categoryName": cname})
		}
		c.Header("Content-Type", "application/json")
		_ = json.NewEncoder(c.Writer).Encode(list)
	})

	return r
}
