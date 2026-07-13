package storage

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var ErrUnknownDriver = errors.New("unknown gorm driver")

type Config struct {
	Driver string
	DSN    string
}

func OpenGORM(config Config, options ...gorm.Option) (*gorm.DB, error) {
	driver := strings.TrimSpace(config.Driver)
	dsn := strings.TrimSpace(config.DSN)
	if dsn == "" {
		dsn = defaultDSN(driver)
	}
	switch driver {
	case "mysql":
		return gorm.Open(mysql.Open(dsn), options...)
	case "postgres":
		return gorm.Open(postgres.Open(dsn), options...)
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), options...)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownDriver, driver)
	}
}

func defaultDSN(driver string) string {
	if driver == "sqlite" {
		return ":memory:"
	}
	return ""
}
