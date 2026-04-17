package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	SlowThreshold   time.Duration
}

func defaultOptions() Options {
	return Options{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		SlowThreshold:   500 * time.Millisecond,
	}
}

func New(dsn string) (*gorm.DB, error) {
	return NewWithOptions(dsn, defaultOptions())
}

func NewWithOptions(dsn string, opts Options) (*gorm.DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             opts.SlowThreshold,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm.DB(): %w", err)
	}

	sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)

	if err := warmPool(sqlDB, opts.MaxIdleConns); err != nil {
		return nil, fmt.Errorf("warm pool: %w", err)
	}

	return db, nil
}

func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("gorm.DB(): %w", err)
	}
	return sqlDB.PingContext(ctx)
}

func warmPool(sqlDB *sql.DB, n int) error {
	if n <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conns := make([]*sql.Conn, 0, n)
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	for i := 0; i < n; i++ {
		c, err := sqlDB.Conn(ctx)
		if err != nil {
			return fmt.Errorf("acquire conn %d: %w", i, err)
		}
		if err := c.PingContext(ctx); err != nil {
			return fmt.Errorf("ping conn %d: %w", i, err)
		}
		conns = append(conns, c)
	}
	return nil
}
