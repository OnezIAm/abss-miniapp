package routes

import (
	"bank-consolidation/internal/controllers"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(db *gorm.DB, contentFS fs.FS) *gin.Engine {
	inv := controllers.InvoiceController{DB: db}
	txc := controllers.TransactionController{DB: db}
	cat := controllers.CategoryController{DB: db}
	be := controllers.BankEntryController{DB: db}
	bt := controllers.BankTypeController{DB: db}
	sys := controllers.SystemController{}

	r := gin.Default()

	// Serve Next.js static files from embedded FS
	if sub, err := fs.Sub(contentFS, "_next"); err == nil {
		r.StaticFS("/_next", http.FS(sub))
	}
	if sub, err := fs.Sub(contentFS, "themes"); err == nil {
		r.StaticFS("/themes", http.FS(sub))
	}
	if sub, err := fs.Sub(contentFS, "demo"); err == nil {
		r.StaticFS("/demo", http.FS(sub))
	}

	r.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("favicon.ico", http.FS(contentFS))
	})

	// Serve index.html for root
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/banking")
	})

	// Fallback for other routes (HTML files or 404)
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// API check
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Helper to check and serve file
		serveIfFile := func(p string) bool {
			f, err := contentFS.Open(p)
			if err != nil {
				return false
			}
			defer f.Close()
			info, err := f.Stat()
			if err != nil || info.IsDir() {
				return false
			}
			c.FileFromFS(p, http.FS(contentFS))
			return true
		}

		// Try exact match (e.g. /robots.txt)
		if serveIfFile(cleanPath) {
			return
		}

		// Try .html extension (e.g. /banking -> /banking.html)
		if serveIfFile(cleanPath + ".html") {
			return
		}

		// Try index.html in directory
		if serveIfFile(cleanPath + "/index.html") {
			return
		}

		c.FileFromFS("404.html", http.FS(contentFS))
	})

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))
	api := r.Group("/api/v1")

	api.GET("/system/backup", sys.BackupDatabase)

	api.POST("/invoices", func(c *gin.Context) { inv.Create(c.Writer, c.Request) })
	api.POST("/invoices/bulk", func(c *gin.Context) { inv.BulkCreate(c.Writer, c.Request) })
	api.POST("/invoices/seed", func(c *gin.Context) { inv.GenerateSample(c.Writer, c.Request) })
	api.DELETE("/invoices", func(c *gin.Context) { inv.BulkDelete(c.Writer, c.Request) })
	api.GET("/invoices", func(c *gin.Context) { inv.CreateOrList(c.Writer, c.Request) })
	api.GET("/invoices/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/invoices/" + c.Param("id")
		inv.GetByID(c.Writer, c.Request)
	})
	api.DELETE("/invoices/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/invoices/" + c.Param("id")
		inv.Delete(c.Writer, c.Request)
	})

	api.POST("/transactions", func(c *gin.Context) { txc.CreateOrList(c.Writer, c.Request) })
	api.GET("/transactions", func(c *gin.Context) { txc.CreateOrList(c.Writer, c.Request) })
	api.POST("/transactions/:id/categories", func(c *gin.Context) {
		c.Request.URL.Path = "/transactions/" + c.Param("id") + "/categories"
		txc.MapCategories(c.Writer, c.Request)
	})

	api.POST("/categories", func(c *gin.Context) { cat.CreateOrList(c.Writer, c.Request) })
	api.GET("/categories", func(c *gin.Context) { cat.CreateOrList(c.Writer, c.Request) })

	api.POST("/bank-types", func(c *gin.Context) { bt.CreateOrList(c.Writer, c.Request) })
	api.GET("/bank-types", func(c *gin.Context) { bt.CreateOrList(c.Writer, c.Request) })
	api.PUT("/bank-types/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-types/" + c.Param("id")
		bt.Update(c.Writer, c.Request)
	})

	// Bank entries CRUD
	api.GET("/bank-entries", func(c *gin.Context) { be.CreateOrList(c.Writer, c.Request) })
	api.POST("/bank-entries", func(c *gin.Context) { be.CreateOrList(c.Writer, c.Request) })
	api.PUT("/bank-entries/finalize", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/bank-entries/finalize"
		be.CreateOrList(c.Writer, c.Request)
	})

	api.PUT("/bank-entries/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id")
		be.Update(c.Writer, c.Request)
	})
	api.DELETE("/bank-entries/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id")
		be.Delete(c.Writer, c.Request)
	})
	api.DELETE("/bank-entries", func(c *gin.Context) {
		be.BulkDelete(c.Writer, c.Request)
	})

	api.POST("/bank-entries/bulk", func(c *gin.Context) { be.BulkCreate(c.Writer, c.Request) })
	api.POST("/bank-entries/seed", func(c *gin.Context) { be.GenerateSample(c.Writer, c.Request) })
	api.POST("/bank-entries/upload", func(c *gin.Context) { be.UploadCSV(c.Writer, c.Request) })

	api.POST("/bank-entries/:id/reconcile", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id") + "/reconcile"
		be.Reconcile(c.Writer, c.Request)
	})
	api.GET("/bank-entries/:id/invoices", func(c *gin.Context) {
		c.Request.URL.Path = "/bank-entries/" + c.Param("id") + "/invoices"
		be.ListAttachedInvoices(c.Writer, c.Request)
	})

	api.GET("/bank-entries/export/reconciled", func(c *gin.Context) {
		be.ExportReconciled(c.Writer, c.Request)
	})

	return r
}
