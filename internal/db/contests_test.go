package db_test

import (
	"errors"
	"testing"
	"time"

	"cotests/internal/db"
	"cotests/internal/testutil"

	"gorm.io/gorm"
)

func TestCreateContestDefaultsToDraft(t *testing.T) {
	database := testutil.NewDatabase(t)
	contest := &db.Contest{Title: "  Spring Contest  ", Description: "  Practice problems  "}

	if err := db.CreateContest(database, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	if contest.Title != "Spring Contest" || contest.Description != "Practice problems" {
		t.Fatalf("contest = %#v, want trimmed fields", contest)
	}
	if contest.Visibility != db.ContestDraft {
		t.Fatalf("visibility = %q, want %q", contest.Visibility, db.ContestDraft)
	}
}

func TestCreateContestValidatesInput(t *testing.T) {
	database := testutil.NewDatabase(t)
	start := time.Now()
	end := start.Add(-time.Hour)

	tests := []struct {
		name    string
		contest db.Contest
	}{
		{"missing title", db.Contest{}},
		{"invalid visibility", db.Contest{Title: "Contest", Visibility: "private"}},
		{"invalid dates", db.Contest{Title: "Contest", StartAt: &start, EndAt: &end}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.CreateContest(database, &tt.contest); err == nil {
				t.Fatal("create contest succeeded, want validation error")
			}
		})
	}
}

func TestSeriesAreOrderedAndPositionsAreUniquePerContest(t *testing.T) {
	database := testutil.NewDatabase(t)
	first := createContest(t, database, "First")
	second := createContest(t, database, "Second")

	for _, series := range []*db.Series{
		{ContestID: first.ID, Title: "Second", Position: 2},
		{ContestID: first.ID, Title: "First", Position: 0},
		{ContestID: second.ID, Title: "Also first", Position: 0},
	} {
		if err := db.CreateSeries(database, series); err != nil {
			t.Fatalf("create series: %v", err)
		}
	}

	ordered, err := db.ListSeries(database, first.ID)
	if err != nil {
		t.Fatalf("list series: %v", err)
	}
	if len(ordered) != 2 || ordered[0].Title != "First" || ordered[1].Title != "Second" {
		t.Fatalf("ordered series = %#v", ordered)
	}

	if err := db.CreateSeries(database, &db.Series{ContestID: first.ID, Title: "Duplicate", Position: 0}); err == nil {
		t.Fatal("duplicate position was accepted")
	}
}

func TestCreateSeriesRequiresExistingContest(t *testing.T) {
	database := testutil.NewDatabase(t)
	series := &db.Series{ContestID: 999, Title: "Orphan", Position: 0}

	if err := db.CreateSeries(database, series); err == nil {
		t.Fatal("series without an existing contest was accepted")
	}
}

func TestCreateSeriesValidatesInput(t *testing.T) {
	database := testutil.NewDatabase(t)
	contest := createContest(t, database, "Contest")

	tests := []struct {
		name   string
		series db.Series
	}{
		{"missing contest", db.Series{Title: "Series", Position: 0}},
		{"missing title", db.Series{ContestID: contest.ID, Position: 0}},
		{"negative position", db.Series{ContestID: contest.ID, Title: "Series", Position: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.CreateSeries(database, &tt.series); err == nil {
				t.Fatal("create series succeeded, want validation error")
			}
		})
	}
}

func TestDeleteContestCascadesToSeries(t *testing.T) {
	database := testutil.NewDatabase(t)
	contest := createContest(t, database, "Contest")
	series := &db.Series{ContestID: contest.ID, Title: "Series", Position: 0}
	if err := db.CreateSeries(database, series); err != nil {
		t.Fatalf("create series: %v", err)
	}

	if err := database.Delete(&db.Contest{}, contest.ID).Error; err != nil {
		t.Fatalf("delete contest: %v", err)
	}
	var count int64
	if err := database.Model(&db.Series{}).Where("contest_id = ?", contest.ID).Count(&count).Error; err != nil {
		t.Fatalf("count series: %v", err)
	}
	if count != 0 {
		t.Fatalf("series count = %d, want 0", count)
	}
}

func TestContestCRUD(t *testing.T) {
	database := testutil.NewDatabase(t)
	first := createContest(t, database, "First")
	second := createContest(t, database, "Second")

	contests, err := db.ListContests(database)
	if err != nil {
		t.Fatalf("list contests: %v", err)
	}
	if len(contests) != 2 || contests[0].ID != second.ID || contests[1].ID != first.ID {
		t.Fatalf("contests = %#v, want reverse creation order", contests)
	}

	update := &db.Contest{ID: first.ID, Title: "Updated", Description: "New description", Visibility: db.ContestPublished}
	if err := db.UpdateContest(database, update); err != nil {
		t.Fatalf("update contest: %v", err)
	}
	loaded, err := db.GetContest(database, first.ID)
	if err != nil {
		t.Fatalf("get contest: %v", err)
	}
	if loaded.Title != "Updated" || loaded.Description != "New description" || loaded.Visibility != db.ContestPublished {
		t.Fatalf("updated contest = %#v", loaded)
	}

	if err := db.DeleteContest(database, first.ID); err != nil {
		t.Fatalf("delete contest: %v", err)
	}
	if _, err := db.GetContest(database, first.ID); err == nil {
		t.Fatal("deleted contest was returned")
	}
}

func TestSeriesCRUD(t *testing.T) {
	database := testutil.NewDatabase(t)
	contest := createContest(t, database, "Contest")
	series := &db.Series{ContestID: contest.ID, Title: "Original", Position: 0}
	if err := db.CreateSeries(database, series); err != nil {
		t.Fatalf("create series: %v", err)
	}

	update := &db.Series{ID: series.ID, Title: "Updated", Position: 1}
	if err := db.UpdateSeries(database, update); err != nil {
		t.Fatalf("update series: %v", err)
	}
	loaded, err := db.GetSeries(database, series.ID)
	if err != nil {
		t.Fatalf("get series: %v", err)
	}
	if loaded.Title != "Updated" || loaded.Position != 1 || loaded.ContestID != contest.ID {
		t.Fatalf("updated series = %#v", loaded)
	}

	if err := db.DeleteSeries(database, series.ID); err != nil {
		t.Fatalf("delete series: %v", err)
	}
	if _, err := db.GetSeries(database, series.ID); err == nil {
		t.Fatal("deleted series was returned")
	}
}

func TestPublicContestsRespectVisibilityAndSchedule(t *testing.T) {
	database := testutil.NewDatabase(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	open := &db.Contest{Title: "Open", Visibility: db.ContestPublished, StartAt: &past, EndAt: &future}
	for _, contest := range []*db.Contest{
		open,
		{Title: "Draft", Visibility: db.ContestDraft},
		{Title: "Future", Visibility: db.ContestPublished, StartAt: &future},
		{Title: "Expired", Visibility: db.ContestPublished, EndAt: &past},
	} {
		if err := db.CreateContest(database, contest); err != nil {
			t.Fatalf("create contest %q: %v", contest.Title, err)
		}
	}

	contests, err := db.ListPublicContests(database, now)
	if err != nil {
		t.Fatalf("list public contests: %v", err)
	}
	if len(contests) != 1 || contests[0].ID != open.ID {
		t.Fatalf("public contests = %#v", contests)
	}
	if _, err := db.GetPublicContest(database, open.ID, now); err != nil {
		t.Fatalf("get open contest: %v", err)
	}
	if _, err := db.GetPublicContest(database, open.ID+1, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("get hidden contest: %v, want record not found", err)
	}
}

func createContest(t *testing.T, database *gorm.DB, title string) *db.Contest {
	t.Helper()
	contest := &db.Contest{Title: title}
	if err := db.CreateContest(database, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	return contest
}
