package db_test

import (
	"path/filepath"
	"testing"

	"cotests/internal/db"
)

func TestSQLiteConstraintsSurviveConnectionReplacement(t *testing.T) {
	for _, options := range []string{"", "?_pragma=busy_timeout(1234)&_pragma=foreign_keys(0)&_txlock=immediate"} {
		t.Run("dsn"+options, func(t *testing.T) {
			database, err := db.Open(filepath.Join(t.TempDir(), "contests.db") + options)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := database.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { pool.Close() })
			if err := db.Migrate(database); err != nil {
				t.Fatal(err)
			}
			contest := createContest(t, database, "Contest")
			if err := db.CreateSeries(database, &db.Series{ContestID: contest.ID, Title: "Series"}); err != nil {
				t.Fatal(err)
			}

			// Force the same replacement that idle expiry can cause in production.
			pool.SetMaxIdleConns(0)
			pool.SetMaxIdleConns(1)
			if options != "" {
				var timeout int
				if err := database.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil || timeout != 1234 {
					t.Fatalf("caller busy_timeout = %d, err = %v", timeout, err)
				}
			}
			if err := db.DeleteContest(database, contest.ID); err != nil {
				t.Fatal(err)
			}
			series, err := db.ListSeries(database, contest.ID)
			if err != nil || len(series) != 0 {
				t.Fatalf("series after parent deletion = %v, err = %v", series, err)
			}
			if err := db.CreateSeries(database, &db.Series{ContestID: contest.ID, Title: "Orphan"}); err == nil {
				t.Fatal("replacement connection accepted an invalid parent")
			}
		})
	}
}
