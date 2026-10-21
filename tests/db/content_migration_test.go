package db_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cotests/internal/db"
)

func TestAccessMigrationPreservesVersionFiveContent(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Load a frozen historical schema independently of production migrations.
	fixture, err := os.ReadFile("testdata/schema_v5.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(fixture), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := database.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	user, err := db.CreateAdmin(database, "upgrade@example.test", "password1", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	contest := db.Contest{Title: "Existing", Visibility: db.ContestPublished}
	if err := database.Omit("Series").Create(&contest).Error; err != nil {
		t.Fatal(err)
	}
	series := db.Series{ContestID: contest.ID, Title: "Existing round", Position: 0}
	if err := database.Omit("Contest", "StartAt", "EndAt", "ReleaseAt").Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	problem := &db.Problem{OwnerID: user.ID, Title: "Existing task", Code: "old"}
	revision := &db.ProblemRevision{Statement: "Keep me"}
	if err := db.CreateProblem(database, problem, revision); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetSeries(database, series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != series.Title || got.StartAt != nil || got.ReleaseAt != nil {
		t.Fatalf("changed existing series: %#v", got)
	}
	restored, err := db.GetProblemRevision(database, revision.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Statement != "Keep me" {
		t.Fatal("content changed during access migration")
	}
	if !database.Migrator().HasTable(&db.ContestMembership{}) || !database.Migrator().HasTable(&db.ImportPreview{}) {
		t.Fatal("new schema missing")
	}
	if err := db.PublishRevision(database, revision.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	second := &db.Series{ContestID: contest.ID, Title: "Another", Position: 1}
	if err := db.CreateSeries(database, second); err != nil {
		t.Fatal(err)
	}
	for _, seriesID := range []uint{series.ID, second.ID} {
		assignment := &db.SeriesProblem{SeriesID: seriesID, ProblemID: problem.ID, PublishedRevisionID: revision.ID, Code: "A", Position: 0}
		if err := database.Omit("Series", "Problem", "PublishedRevision").Create(assignment).Error; err != nil {
			t.Fatalf("code was not scoped to its series: %v", err)
		}
	}
}
