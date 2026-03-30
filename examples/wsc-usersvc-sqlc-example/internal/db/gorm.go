package pg

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type GormProvider struct {
	db *gorm.DB
}

func NewGormProvider(cfg Config) *GormProvider {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormlogger.Config{
				LogLevel:                  gormlogger.Warn,
				IgnoreRecordNotFoundError: true,
			},
		),
	})
	if err != nil {
		log.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Successfully connected to the database with GORM")

	return &GormProvider{db: db}
}

func (p *GormProvider) DB() *gorm.DB {
	return p.db
}
