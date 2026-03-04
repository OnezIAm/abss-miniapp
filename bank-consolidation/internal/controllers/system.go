package controllers

import (
	"bank-consolidation/internal/config"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

type SystemController struct{}

func (c SystemController) BackupDatabase(ctx *gin.Context) {
	dataDir := config.ResolveDataDir()
	dbPath := filepath.Join(dataDir, "bank.db")

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "Database file not found",
		})
		return
	}

	// Generate backup filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("bank_backup_%s.db", timestamp)

	// Set headers for file download
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	ctx.Header("Content-Type", "application/x-sqlite3")
	ctx.Header("Cache-Control", "no-cache")

	// Stream the file
	ctx.File(dbPath)
}
