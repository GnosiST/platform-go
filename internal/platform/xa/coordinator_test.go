package xa

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestCoordinatorIsDisabledByDefault(t *testing.T) {
	coordinator, err := NewCoordinator(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Execute(context.Background(), "orders-reconcile"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Execute() error = %v, want ErrDisabled", err)
	}
}

func TestGORMRecoveryLogPersistsRecords(t *testing.T) {
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/xa.db"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMRecoveryLog(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	log, err := OpenGORMRecoveryLog(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	record := RecoveryRecord{TransactionID: "xa_0123456789abcdef0123456789abcdef", Operation: "orders-reconcile", Status: StatusHeuristic, ResourceManagers: []string{"orders", "billing"}, OccurredAt: time.Now().UTC()}
	if err := log.Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenGORMRecoveryLog(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	records, err := reopened.Records(context.Background())
	if err != nil || len(records) != 1 || records[0].Status != StatusHeuristic || len(records[0].ResourceManagers) != 2 {
		t.Fatalf("Records() = %+v, %v", records, err)
	}
}

func TestCoordinatorRecordsHeuristicFailureForRecovery(t *testing.T) {
	log := NewMemoryRecoveryLog()
	coordinator, err := NewCoordinator(Config{Enabled: true, Timeout: time.Second}, []ResourceManager{
		managerStub{id: "orders", commitErr: errors.New("commit unavailable")},
		managerStub{id: "billing"},
	}, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Execute(context.Background(), "orders-reconcile"); !errors.Is(err, ErrHeuristic) {
		t.Fatalf("Execute() error = %v, want ErrHeuristic", err)
	}
	records := log.Records()
	if len(records) != 1 || records[0].Status != StatusHeuristic || len(records[0].ResourceManagers) != 2 {
		t.Fatalf("recovery records = %+v", records)
	}
}

func TestMySQLXACertificationSmoke(t *testing.T) {
	first := os.Getenv("PLATFORM_CERTIFICATION_XA_MYSQL_URL_ONE")
	second := os.Getenv("PLATFORM_CERTIFICATION_XA_MYSQL_URL_TWO")
	if first == "" || second == "" {
		t.Skip("XA MySQL certification URLs are not configured")
	}
	firstManager, err := OpenMySQLResourceManager("mysql-one", first)
	if err != nil {
		t.Fatal(err)
	}
	defer firstManager.Close()
	secondManager, err := OpenMySQLResourceManager("mysql-two", second)
	if err != nil {
		t.Fatal(err)
	}
	defer secondManager.Close()
	coordinator, err := NewCoordinator(Config{Enabled: true, Timeout: 10 * time.Second}, []ResourceManager{firstManager, secondManager}, NewMemoryRecoveryLog())
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Execute(context.Background(), "mysql-xa-certification"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

type managerStub struct {
	id         string
	prepareErr error
	commitErr  error
}

func (m managerStub) ID() string                             { return m.id }
func (m managerStub) Prepare(context.Context, string) error  { return m.prepareErr }
func (m managerStub) Commit(context.Context, string) error   { return m.commitErr }
func (m managerStub) Rollback(context.Context, string) error { return nil }
