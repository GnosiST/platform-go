package xa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var transactionIDPattern = regexp.MustCompile(`^xa_[0-9a-f]{32}$`)

// MySQLResourceManager executes the MySQL XA protocol. It is intended only
// for an explicitly enabled coordinator and never replaces local transactions.
type MySQLResourceManager struct {
	id string
	db *sql.DB
}

func OpenMySQLResourceManager(id, dsn string) (*MySQLResourceManager, error) {
	if !identifierPattern.MatchString(id) || strings.TrimSpace(dsn) == "" {
		return nil, ErrInvalidConfig
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql xa resource manager: %w", err)
	}
	return &MySQLResourceManager{id: id, db: db}, nil
}

func (m *MySQLResourceManager) ID() string { return m.id }

func (m *MySQLResourceManager) Prepare(ctx context.Context, transactionID string) error {
	if m == nil || m.db == nil || ctx == nil || !transactionIDPattern.MatchString(transactionID) {
		return ErrInvalidConfig
	}
	for _, statement := range []string{"XA START '" + transactionID + "'", "XA END '" + transactionID + "'", "XA PREPARE '" + transactionID + "'"} {
		if _, err := m.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("mysql xa prepare: %w", err)
		}
	}
	return nil
}

func (m *MySQLResourceManager) Commit(ctx context.Context, transactionID string) error {
	return m.finish(ctx, "COMMIT", transactionID)
}

func (m *MySQLResourceManager) Rollback(ctx context.Context, transactionID string) error {
	return m.finish(ctx, "ROLLBACK", transactionID)
}

func (m *MySQLResourceManager) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m *MySQLResourceManager) finish(ctx context.Context, action, transactionID string) error {
	if m == nil || m.db == nil || ctx == nil || !transactionIDPattern.MatchString(transactionID) {
		return ErrInvalidConfig
	}
	if action != "COMMIT" && action != "ROLLBACK" {
		return errors.New("invalid xa finish action")
	}
	if _, err := m.db.ExecContext(ctx, "XA "+action+" '"+transactionID+"'"); err != nil {
		return fmt.Errorf("mysql xa %s: %w", strings.ToLower(action), err)
	}
	return nil
}
