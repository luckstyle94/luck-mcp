package migrations

import (
	"context"
	"regexp"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRunnerRun_AppliesOnlyPendingMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	filesystem := fstest.MapFS{
		"0001_init.up.sql": {Data: []byte("CREATE TABLE test_one(id INT);")},
		"0002_more.up.sql": {Data: []byte("ALTER TABLE test_one ADD COLUMN name TEXT;")},
	}

	firstChecksum := checksum("CREATE TABLE test_one(id INT);")
	secondChecksum := checksum("ALTER TABLE test_one ADD COLUMN name TEXT;")

	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_lock($1)`)).
		WithArgs(advisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
CREATE TABLE IF NOT EXISTS luck_mcp_schema_migrations (
	version TEXT PRIMARY KEY,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT version, checksum
FROM luck_mcp_schema_migrations
ORDER BY version`)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow("0001_init.up.sql", firstChecksum))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE test_one ADD COLUMN name TEXT;")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO luck_mcp_schema_migrations (version, checksum)
VALUES ($1, $2)`)).
		WithArgs("0002_more.up.sql", secondChecksum).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_unlock($1)`)).
		WithArgs(advisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 1))

	runner := NewRunnerWithFS(db, nil, filesystem)
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected 1 skipped migration, got %d", result.Skipped)
	}
	if len(result.Applied) != 1 || result.Applied[0] != "0002_more.up.sql" {
		t.Fatalf("unexpected applied migrations: %#v", result.Applied)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRunnerRun_FailsOnChecksumMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	filesystem := fstest.MapFS{
		"0001_init.up.sql": {Data: []byte("CREATE TABLE test_one(id INT);")},
	}

	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_lock($1)`)).
		WithArgs(advisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
CREATE TABLE IF NOT EXISTS luck_mcp_schema_migrations (
	version TEXT PRIMARY KEY,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT version, checksum
FROM luck_mcp_schema_migrations
ORDER BY version`)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow("0001_init.up.sql", "different-checksum"))
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_unlock($1)`)).
		WithArgs(advisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 1))

	runner := NewRunnerWithFS(db, nil, filesystem)
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func checksum(sql string) string {
	filesystem := fstest.MapFS{
		"0001_tmp.up.sql": {Data: []byte(sql)},
	}
	migrations, err := loadMigrationFiles(filesystem)
	if err != nil {
		panic(err)
	}
	return migrations[0].Checksum
}
