package store_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tormgibbs/pulse-check-api/internal/domain"
	"github.com/tormgibbs/pulse-check-api/internal/store"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Printf("failed to terminate container: %v", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("failed to get connection string: %v", err)
	}

	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer testPool.Close()

	if err := testPool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "..", "db", "migrations")

	migrator, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		connStr,
	)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}
	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to run migrations: %v", err)
	}

	os.Exit(m.Run())
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testPool
}

func newTestMonitor(id string) *domain.Monitor {
	return &domain.Monitor{
		ID:                id,
		TimeoutSeconds:    30,
		FailureThreshold:  2,
		RecoveryThreshold: 3,
		RecoveryWindow:    90,
		AlertOnRecovery:   true,
		Status:            domain.StatusActive,
		LastHeartbeatAt:   time.Now().UTC(),
	}
}

func TestCreate_Success(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	m := newTestMonitor("device-01")
	if err := s.Create(context.Background(), m); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCreate_Duplicate(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	m := newTestMonitor("device-dup")
	if err := s.Create(context.Background(), m); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	err := s.Create(context.Background(), m)
	if err != domain.ErrDuplicateMonitor {
		t.Errorf("expected ErrDuplicateMonitor, got %v", err)
	}
}

func TestGetByID_Success(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	m := newTestMonitor("device-02")
	if err := s.Create(context.Background(), m); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	got, err := s.GetByID(context.Background(), "device-02")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != "device-02" {
		t.Errorf("expected id device-02, got %s", got.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	_, err := s.GetByID(context.Background(), "nonexistent")
	if err != domain.ErrMonitorNotFound {
		t.Errorf("expected ErrMonitorNotFound, got %v", err)
	}
}

func TestUpdateHeartbeat_ResetsFailureCount(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	m := newTestMonitor("device-03")
	if err := s.Create(context.Background(), m); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := s.UpdateFailureCount(context.Background(), "device-03", 2); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	now := time.Now().UTC()
	if err := s.UpdateHeartbeat(context.Background(), "device-03", now); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := s.GetByID(context.Background(), "device-03")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.FailureCount != 0 {
		t.Errorf("expected failure_count 0, got %d", got.FailureCount)
	}
}

func TestUpdateStatus_TransitionsToPaused(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	m := newTestMonitor("device-04")
	if err := s.Create(context.Background(), m); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := s.UpdateStatus(context.Background(), "device-04", domain.StatusPaused, nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := s.GetByID(context.Background(), "device-04")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Status != domain.StatusPaused {
		t.Errorf("expected status paused, got %s", got.Status)
	}
}

func TestDelete_Success(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	m := newTestMonitor("device-05")
	if err := s.Create(context.Background(), m); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := s.Delete(context.Background(), "device-05"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err := s.GetByID(context.Background(), "device-05")
	if err != domain.ErrMonitorNotFound {
		t.Errorf("expected ErrMonitorNotFound after delete, got %v", err)
	}
}

func TestListRecoverable_ExcludesDownMonitors(t *testing.T) {
	pool := setupTestDB(t)
	s := store.NewMonitorStore(pool)

	active := newTestMonitor("device-06")
	down := newTestMonitor("device-07")

	if err := s.Create(context.Background(), active); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := s.Create(context.Background(), down); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	now := time.Now().UTC()
	if err := s.UpdateStatus(context.Background(), "device-07", domain.StatusDown, &now); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	monitors, err := s.ListRecoverable(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, m := range monitors {
		if m.ID == "device-07" {
			t.Error("expected down monitor to be excluded from ListRecoverable")
		}
	}
}
