//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/database"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/migrations"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
)

var integrationDB *bun.DB

// testContext deliberately stays compatible with the module's Go 1.23
// baseline (testing.T.Context was added later).
func testContext(_ *testing.T) context.Context { return context.Background() }

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:17-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_DB":       "bookflow_integration",
				"POSTGRES_USER":     "bookflow",
				"POSTGRES_PASSWORD": "bookflow-integration",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "start PostgreSQL testcontainer: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(context.Background())
		fmt.Fprintf(os.Stderr, "resolve PostgreSQL testcontainer host: %v\n", err)
		os.Exit(1)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(context.Background())
		fmt.Fprintf(os.Stderr, "resolve PostgreSQL testcontainer port: %v\n", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf("postgres://bookflow:bookflow-integration@%s:%s/bookflow_integration?sslmode=disable", host, port.Port())
	integrationDB, err = database.Open(ctx, config.Database{
		URL:             dsn,
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Minute,
		PingTimeout:     10 * time.Second,
	})
	if err != nil {
		_ = container.Terminate(context.Background())
		fmt.Fprintf(os.Stderr, "open integration database: %v\n", err)
		os.Exit(1)
	}
	if err = migrations.Up(ctx, integrationDB); err != nil {
		_ = integrationDB.Close()
		_ = container.Terminate(context.Background())
		fmt.Fprintf(os.Stderr, "apply embedded migrations: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = integrationDB.Close()
	terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer terminateCancel()
	if err := container.Terminate(terminateCtx); err != nil {
		fmt.Fprintf(os.Stderr, "terminate PostgreSQL testcontainer: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func resetDatabase(t *testing.T) {
	t.Helper()
	_, err := integrationDB.ExecContext(testContext(t), `
DO $$
DECLARE item record;
BEGIN
  FOR item IN
    SELECT tablename
    FROM pg_tables
    WHERE schemaname = 'public' AND tablename <> 'schema_migrations'
  LOOP
    EXECUTE format('TRUNCATE TABLE %I CASCADE', item.tablename);
  END LOOP;
END $$;`)
	if err != nil {
		t.Fatalf("reset integration database: %v", err)
	}
}

func createUser(t *testing.T, timezone string) model.User {
	t.Helper()
	if timezone == "" {
		timezone = "UTC"
	}
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	user := model.User{
		ID:           uuid.New(),
		Email:        uuid.NewString() + "@example.test",
		PasswordHash: "integration-test-password-hash",
		DisplayName:  "Integration Test",
		Timezone:     timezone,
		Locale:       "en",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := integrationDB.NewInsert().Model(&user).Exec(testContext(t)); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func createBook(t *testing.T, userID uuid.UUID, title string) model.Book {
	t.Helper()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	book := model.Book{
		ID:                uuid.New(),
		UserID:            userID,
		Title:             title,
		Author:            "Test Author",
		Language:          "en",
		Format:            "txt",
		Status:            "ready",
		SHA256:            fmt.Sprintf("%064x", uuid.New().ID()),
		OriginalFilename:  "book.txt",
		OriginalMIME:      "text/plain",
		OriginalSize:      1024,
		OriginalBucket:    "books-original",
		OriginalKey:       "integration/" + uuid.NewString(),
		ProcessingVersion: 1,
		Metadata:          []byte(`{}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if _, err := integrationDB.NewInsert().Model(&book).Exec(testContext(t)); err != nil {
		t.Fatalf("create book: %v", err)
	}
	return book
}

func createChapter(t *testing.T, bookID uuid.UUID, ordinal int) model.BookChapter {
	t.Helper()
	chapter := model.BookChapter{
		ID:             uuid.New(),
		BookID:         bookID,
		Version:        1,
		Ordinal:        ordinal,
		Title:          fmt.Sprintf("Chapter %d", ordinal+1),
		SourceRef:      fmt.Sprintf("chapter-%d", ordinal+1),
		ContentHTML:    "<p>Integration chapter.</p>",
		ContentText:    "Integration chapter.",
		CharacterCount: 20,
		WordCount:      2,
		CreatedAt:      time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	if _, err := integrationDB.NewInsert().Model(&chapter).Exec(testContext(t)); err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	return chapter
}

func createDevice(t *testing.T, userID uuid.UUID) model.Device {
	t.Helper()
	device := model.Device{
		ID:         uuid.New(),
		UserID:     userID,
		DeviceKey:  uuid.NewString(),
		Name:       "Integration browser",
		UserAgent:  "bookflow-integration-tests",
		LastSeenAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		CreatedAt:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	if _, err := integrationDB.NewInsert().Model(&device).Exec(testContext(t)); err != nil {
		t.Fatalf("create device: %v", err)
	}
	return device
}
