package routes

import (
	"bank-consolidation/internal/controllers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(db *gorm.DB) *gin.Engine {
	inv := controllers.InvoiceController{DB: db}
	txc := controllers.TransactionController{DB: db}
	cat := controllers.CategoryController{DB: db}
	be := controllers.BankEntryController{DB: db}
	rpt := controllers.ReportsController{DB: db}

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
		rpt.GetInvoices(c.Writer, c.Request)
	})

	api.GET("/reports/transactions/categories", func(c *gin.Context) {
		rpt.GetTransactionCategories(c.Writer, c.Request)
	})

	return r
}
