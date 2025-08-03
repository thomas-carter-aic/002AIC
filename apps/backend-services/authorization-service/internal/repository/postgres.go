package repository

import (
	"fmt"
	"github.com/002aic/authorization-service/internal/config"
	"github.com/002aic/authorization-service/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PolicyRepository interface {
	GetDB() *gorm.DB
}

type policyRepository struct {
	db *gorm.DB
}

func NewPostgresDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.Policy{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func NewPolicyRepository(db *gorm.DB) PolicyRepository {
	return &policyRepository{db: db}
}

func (r *policyRepository) GetDB() *gorm.DB {
	return r.db
}

// PingDB checks if the database connection is alive
func PingDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
