package db

import (
	"go-mac/internal/models"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(path string) {
	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// Auto-migrate models
	err = DB.AutoMigrate(&models.Switch{}, &models.PortStatus{}, &models.MacEntry{})
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Backfill empty site values
	if err := DB.Model(&models.Switch{}).
		Where("site IS NULL OR site = ''").
		Updates(map[string]interface{}{"site": "default"}).Error; err != nil {
		log.Printf("Failed to backfill Switch.site: %v", err)
	}
}
