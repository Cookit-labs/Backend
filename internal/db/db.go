package db

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Cookit-labs/Backend/internal/models"
)

type DB struct {
	*gorm.DB
}

func New(dbURL string) (*DB, error) {
	dialector := postgres.Open(dbURL)
	gormdb, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &DB{gormdb}, nil
}

// Migrate runs all database migrations
func (db *DB) Migrate() error {
	tables := []any{
		&models.Agent{},
		&models.AgentReputation{},
		&models.Intent{},
		&models.Proposal{},
		&models.Execution{},
	}

	for _, table := range tables {
		if err := db.AutoMigrate(table); err != nil {
			return fmt.Errorf("migration failed for %T: %w", table, err)
		}
		log.Printf("✓ migrated %T", table)
	}
	return nil
}

// Health checks database connectivity
func (db *DB) Health() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
