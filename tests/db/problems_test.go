package db_test

import (
	"testing"
	"time"

	"cotests/internal/db"
	"cotests/tests/internal/testutil"
)

func TestPublishedProblemRevisionIsImmutable(t *testing.T) {
	database := testutil.NewDatabase(t)
	owner := testutil.CreateAdmin(t, database, "author@example.com")
	problem := &db.Problem{OwnerID: owner.ID, Title: "Sum", Code: "sum"}
	revision := &db.ProblemRevision{Statement: "Add two integers."}
	if err := db.CreateProblem(database, problem, revision); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	if revision.Number != 1 || revision.State != db.RevisionDraft {
		t.Fatalf("initial revision = %#v", revision)
	}
	if err := db.PublishRevision(database, revision.ID, time.Now()); err != nil {
		t.Fatalf("publish revision: %v", err)
	}
	revision.State = db.RevisionDraft
	revision.Statement = "Changed"
	if err := db.UpdateDraftRevision(database, revision); err == nil {
		t.Fatal("published revision was editable")
	}
	draft, err := db.CreateDraftRevision(database, problem.ID)
	if err != nil {
		t.Fatalf("create next draft: %v", err)
	}
	if draft.Number != 2 || draft.Statement != "Add two integers." {
		t.Fatalf("draft = %#v", draft)
	}
}
