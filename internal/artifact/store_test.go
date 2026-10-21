package artifact

import (
	"os"
	"strings"
	"testing"

	"cotests/internal/testutil"
)

func TestStoreKeepsArtifactsPrivateAndBounded(t *testing.T) {
	database := testutil.NewDatabase(t)
	owner := testutil.CreateAdmin(t, database, "author@example.com")
	store := Store{Root: t.TempDir(), MaxBytes: 3}
	artifact, err := store.Save(database, owner.ID, "../input.txt", "text/plain", strings.NewReader("abc"))
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if artifact.Filename != "input.txt" {
		t.Fatalf("filename = %q", artifact.Filename)
	}
	if _, err := os.Stat(store.Root + "/" + artifact.StorageKey); err != nil {
		t.Fatalf("private file missing: %v", err)
	}
	if _, err := store.Save(database, owner.ID, "large.txt", "text/plain", strings.NewReader("abcd")); err == nil {
		t.Fatal("oversized artifact was accepted")
	}
}
