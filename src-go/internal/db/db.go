package db

import (
	"fmt"
	"log/slog"

	"github.com/omnivore-app/omnivore/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// DB holds primary and optional read-replica GORM connections.
type DB struct {
	Write *gorm.DB
	Read  *gorm.DB // same as Write when no replica is configured
}

// New opens the primary (and optionally replica) database connections.
// The omnivore PostgreSQL schema is set via search_path in the DSN.
func New(cfg *config.Config) (*DB, error) {
	write, err := open(dsn(
		cfg.PGHost, cfg.PGPort, cfg.PGUser, cfg.PGPassword,
		cfg.PGDatabase, cfg.PGPoolMax,
	))
	if err != nil {
		return nil, fmt.Errorf("open primary db: %w", err)
	}

	read := write
	if cfg.PGReplicaHost != "" {
		replicaUser := cfg.PGReplicaUser
		if replicaUser == "" {
			replicaUser = cfg.PGUser
		}
		replicaPassword := cfg.PGReplicaPassword
		if replicaPassword == "" {
			replicaPassword = cfg.PGPassword
		}
		replicaDB := cfg.PGReplicaDatabase
		if replicaDB == "" {
			replicaDB = cfg.PGDatabase
		}
		read, err = open(dsn(
			cfg.PGReplicaHost, cfg.PGReplicaPort,
			replicaUser, replicaPassword,
			replicaDB, cfg.PGPoolMax,
		))
		if err != nil {
			return nil, fmt.Errorf("open replica db: %w", err)
		}
	}

	slog.Info("database connected", "host", cfg.PGHost)
	return &DB{Write: write, Read: read}, nil
}

func dsn(host string, port int, user, password, dbname string, poolMax int) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s search_path=omnivore sslmode=disable pool_max_conns=%d connect_timeout=10",
		host, port, user, password, dbname, poolMax,
	)
}

func open(dsn string) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NamingStrategy: schema.NamingStrategy{
			// All tables live in the omnivore schema; GORM prepends "omnivore."
			// when TablePrefix is set, matching TypeORM's snake_case behaviour.
			TablePrefix:   "omnivore.",
			SingularTable: false,
		},
		// Do not automatically create updated_at on tables that don't have it.
		DisableAutomaticPing: false,
	}

	db, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)

	return db, nil
}

// Close shuts down the underlying sql.DB connections.
func (d *DB) Close() {
	if sqlDB, err := d.Write.DB(); err == nil {
		_ = sqlDB.Close()
	}
	if d.Read != d.Write {
		if sqlDB, err := d.Read.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	slog.Info("database connections closed")
}
