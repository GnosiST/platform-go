package storage

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpenGORMRejectsUnknownDriver(t *testing.T) {
	_, err := OpenGORM(Config{Driver: "mongo", DSN: "memory"})
	if err == nil {
		t.Fatalf("OpenGORM() error = nil, want unknown driver")
	}
}

func TestOpenGORMOpensSQLiteMemory(t *testing.T) {
	db, err := OpenGORM(Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("OpenGORM(sqlite) error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("sqlite ping error = %v", err)
	}
}

func TestOpenGORMCanSilenceInitializationErrors(t *testing.T) {
	const (
		helperEnvironment = "PLATFORM_TEST_SILENT_GORM_OPEN"
		secretMarker      = "sensitive-gorm-open-marker"
	)
	if os.Getenv(helperEnvironment) == "1" {
		_, _ = OpenGORM(
			Config{Driver: "postgres", DSN: "postgres://" + secretMarker + ":password@127.0.0.1:1/platform?sslmode=disable"},
			&gorm.Config{Logger: logger.Discard},
		)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestOpenGORMCanSilenceInitializationErrors$")
	command.Env = append(os.Environ(), helperEnvironment+"=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("helper test error = %v, stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	childOutput := strings.TrimPrefix(stdout.String(), "PASS\n")
	if childOutput != "" || stderr.Len() != 0 || strings.Contains(stdout.String()+stderr.String(), secretMarker) {
		t.Fatalf("gorm.Open output stdout=%q stderr=%q, want silent initialization failure", stdout.String(), stderr.String())
	}
}
