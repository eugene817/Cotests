package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAccessMigrationPreservesVersionFiveContent(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrator().CreateTable(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:5] {
		if err := migration.apply(database); err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&schemaMigration{Version: migration.version, AppliedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	user, err := CreateAdmin(database, "upgrade@example.test", "password1", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	contest := migrationContest{Title: "Existing", Visibility: ContestPublished}
	if err := database.Create(&contest).Error; err != nil {
		t.Fatal(err)
	}
	series := migrationSeries{ContestID: contest.ID, Title: "Existing round", Position: 0}
	if err := database.Omit("Contest").Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	problem := &Problem{OwnerID: user.ID, Title: "Existing task", Code: "old"}
	revision := &ProblemRevision{Statement: "Keep me"}
	if err := CreateProblem(database, problem, revision); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	got, err := GetSeries(database, series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != series.Title || got.StartAt != nil || got.ReleaseAt != nil {
		t.Fatalf("changed existing series: %#v", got)
	}
	restored, err := GetProblemRevision(database, revision.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Statement != "Keep me" {
		t.Fatal("content changed during access migration")
	}
	if !database.Migrator().HasTable(&ContestMembership{}) || !database.Migrator().HasTable(&ImportPreview{}) {
		t.Fatal("new schema missing")
	}
	if err := PublishRevision(database, revision.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	second := &Series{ContestID: contest.ID, Title: "Another", Position: 1}
	if err := CreateSeries(database, second); err != nil {
		t.Fatal(err)
	}
	for _, seriesID := range []uint{series.ID, second.ID} {
		assignment := &SeriesProblem{SeriesID: seriesID, ProblemID: problem.ID, PublishedRevisionID: revision.ID, Code: "A", Position: 0}
		if err := database.Omit("Series", "Problem", "PublishedRevision").Create(assignment).Error; err != nil {
			t.Fatalf("code was not scoped to its series: %v", err)
		}
	}
}
