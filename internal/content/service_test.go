package content_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cotests/internal/artifact"
	"cotests/internal/content"
	"cotests/internal/db"
	"cotests/internal/importer"
	"cotests/internal/testutil"
	"gorm.io/gorm"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	s                                    *content.Service
	admin, author, participant, outsider *db.User
	contest                              *db.Contest
	series                               *db.Series
	problem                              *db.Problem
	revision                             *db.ProblemRevision
	now                                  time.Time
}

func setup(t *testing.T, database *gorm.DB) *fixture {
	t.Helper()
	f := &fixture{now: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	f.s = &content.Service{DB: database, Store: artifact.Store{Root: t.TempDir(), MaxBytes: 4096, MaxTotalBytes: 1 << 20}, ImportLimits: importer.Limits{MaxBytes: 1 << 20, MaxFiles: 100}, Now: func() time.Time { return f.now }}
	f.admin = testutil.CreateAdmin(t, database, "admin@content.test")
	f.author = testutil.CreateUser(t, database, "author@content.test")
	f.participant = testutil.CreateUser(t, database, "participant@content.test")
	f.outsider = testutil.CreateUser(t, database, "outsider@content.test")
	f.contest = &db.Contest{Title: "Contest", Visibility: db.ContestPublished}
	must(t, db.CreateContest(database, f.contest))
	ctx := context.Background()
	must(t, f.s.SetMember(ctx, f.admin, f.contest.ID, f.author.ID, "organizer", false))
	must(t, f.s.SetMember(ctx, f.author, f.contest.ID, f.participant.ID, "participant", false))
	f.series = &db.Series{ContestID: f.contest.ID, Title: "Round", Position: 0}
	must(t, f.s.SaveSeries(ctx, f.author, f.series))
	f.problem = &db.Problem{OwnerID: f.author.ID, Title: "Problem", Code: "P"}
	f.revision = &db.ProblemRevision{Statement: "Statement", LanguageMetadata: `["cpp"]`}
	must(t, db.CreateProblem(database, f.problem, f.revision))
	return f
}

func TestContentDatabaseContract(t *testing.T) {
	for name, factory := range map[string]func(*testing.T) *gorm.DB{"sqlite": testutil.NewDatabase, "postgres": testutil.NewPostgresDatabase} {
		t.Run(name, func(t *testing.T) {
			f := setup(t, factory(t))
			ctx := context.Background()
			t.Run("policy boundaries", func(t *testing.T) {
				start := f.now
				end := start.Add(time.Hour)
				release := start.Add(time.Minute)
				f.contest.StartAt = &start
				f.contest.EndAt = &end
				must(t, db.UpdateContest(f.s.DB, f.contest))
				f.series.StartAt = &start
				f.series.EndAt = &end
				f.series.ReleaseAt = &release
				must(t, f.s.SaveSeries(ctx, f.author, f.series))
				if !errors.Is(f.s.CanReadSeries(ctx, f.participant, f.series), content.ErrForbidden) {
					t.Fatal("early release allowed")
				}
				must(t, f.s.CanReadSeries(ctx, f.author, f.series))
				f.now = release
				must(t, f.s.CanReadSeries(ctx, f.participant, f.series))
				must(t, f.s.CanSubmit(ctx, f.participant, f.series.ID))
				f.now = end
				if !errors.Is(f.s.CanSubmit(ctx, f.participant, f.series.ID), content.ErrForbidden) {
					t.Fatal("end boundary accepted")
				}
				must(t, f.s.CanReadSeries(ctx, f.participant, f.series))
				if !errors.Is(f.s.ManageContest(ctx, f.outsider, f.contest.ID), content.ErrForbidden) {
					t.Fatal("outsider can manage")
				}
				if !errors.Is(f.s.SetMember(ctx, f.author, f.contest.ID, f.author.ID, "participant", false), content.ErrForbidden) {
					t.Fatal("organizer can demote organizer")
				}
				f.series.ReleaseAt = nil
				f.series.StartAt = nil
				f.series.EndAt = nil
				must(t, f.s.SaveSeries(ctx, f.author, f.series))
				f.now = start
			})
			t.Run("publication and clone", func(t *testing.T) {
				sample, err := f.s.SaveTest(ctx, f.author, f.revision.ID, 0, content.TestInput{Position: 0, IsSample: true, Input: "1", Output: "2"})
				must(t, err)
				hidden, err := f.s.SaveTest(ctx, f.author, f.revision.ID, 0, content.TestInput{Position: 1, Input: "secret", Output: "answer"})
				must(t, err)
				pdf, err := f.s.UploadPDF(ctx, f.author, f.revision.ID, strings.NewReader("%PDF-1.7\nstatement"))
				must(t, err)
				must(t, db.PublishRevision(f.s.DB, f.revision.ID, f.now))
				assignment := &db.SeriesProblem{SeriesID: f.series.ID, PublishedRevisionID: f.revision.ID, Code: "A"}
				must(t, f.s.Assign(ctx, f.author, assignment))
				statement, err := f.s.Statement(ctx, f.participant, assignment.ID)
				must(t, err)
				if len(statement.Samples) != 1 || statement.Samples[0].InputArtifactID != sample.InputArtifactID {
					t.Fatal("bad sample projection")
				}
				_, err = f.s.Artifact(ctx, f.participant, sample.InputArtifactID)
				must(t, err)
				_, err = f.s.Artifact(ctx, f.participant, pdf.ID)
				must(t, err)
				if _, err = f.s.Artifact(ctx, f.participant, hidden.InputArtifactID); !errors.Is(err, content.ErrForbidden) {
					t.Fatal("hidden input leaked")
				}
				if _, err = f.s.Artifact(ctx, f.outsider, pdf.ID); !errors.Is(err, content.ErrForbidden) {
					t.Fatal("PDF leaked")
				}
				if err = f.s.DeleteTest(ctx, f.author, f.revision.ID, sample.ID); !errors.Is(err, content.ErrConflict) {
					t.Fatal("published test deleted")
				}
				draft, err := db.CreateDraftRevision(f.s.DB, f.problem.ID)
				must(t, err)
				tests, err := db.ListTestCases(f.s.DB, draft.ID)
				must(t, err)
				if len(tests) != 2 || tests[0].InputArtifactID != sample.InputArtifactID || draft.PDFArtifactID == nil || *draft.PDFArtifactID != pdf.ID {
					t.Fatal("draft did not retain immutable files")
				}
				must(t, f.s.DeleteTest(ctx, f.author, draft.ID, tests[0].ID))
				original, err := db.ListTestCases(f.s.DB, f.revision.ID)
				must(t, err)
				if len(original) != 2 {
					t.Fatal("draft changed publication")
				}
			})
			t.Run("reorder rollback", func(t *testing.T) {
				second := &db.Series{ContestID: f.contest.ID, Title: "Second", Position: 1}
				must(t, f.s.SaveSeries(ctx, f.author, second))
				must(t, f.s.Reorder(ctx, f.author, f.contest.ID, []uint{second.ID, f.series.ID}, false))
				if err := f.s.Reorder(ctx, f.author, f.contest.ID, []uint{f.series.ID, f.series.ID}, false); err == nil {
					t.Fatal("duplicate accepted")
				}
				rows, err := db.ListSeries(f.s.DB, f.contest.ID)
				must(t, err)
				if rows[0].ID != second.ID || rows[0].Position != 0 || rows[1].Position != 1 {
					t.Fatal("failed reorder was not atomic")
				}
			})
			t.Run("concurrent publication", func(t *testing.T) {
				draft, err := db.CreateDraftRevision(f.s.DB, f.problem.ID)
				must(t, err)
				var wg sync.WaitGroup
				errs := make(chan error, 2)
				wg.Add(2)
				go func() { defer wg.Done(); errs <- db.PublishRevision(f.s.DB, draft.ID, f.now) }()
				go func() {
					defer wg.Done()
					_, err := f.s.SaveTest(ctx, f.author, draft.ID, 0, content.TestInput{Position: 10, Input: "x", Output: "y"})
					errs <- err
				}()
				wg.Wait()
				close(errs)
				for err := range errs {
					if err != nil && !errors.Is(err, content.ErrConflict) {
						t.Fatal(err)
					}
				}
				_, err = f.s.SaveTest(ctx, f.author, draft.ID, 0, content.TestInput{Position: 11, Input: "x", Output: "y"})
				if !errors.Is(err, content.ErrConflict) {
					t.Fatal("published revision mutable")
				}
			})
		})
	}
}

func zipBytes(t *testing.T) []byte {
	t.Helper()
	var raw bytes.Buffer
	writer := zip.NewWriter(&raw)
	for _, name := range []string{"a.in", "a.out", "b.in", "b.out"} {
		file, err := writer.Create(name)
		must(t, err)
		_, err = file.Write([]byte("data"))
		must(t, err)
	}
	must(t, writer.Close())
	return raw.Bytes()
}

func TestFileTransactionsAndImports(t *testing.T) {
	for name, factory := range map[string]func(*testing.T) *gorm.DB{"sqlite": testutil.NewDatabase, "postgres": testutil.NewPostgresDatabase} {
		t.Run(name, func(t *testing.T) {
			f := setup(t, factory(t))
			ctx := context.Background()
			// A failure on the second artifact compensates the first write and DB row.
			_, err := f.s.SaveTest(ctx, f.author, f.revision.ID, 0, content.TestInput{Input: "small", Output: strings.Repeat("x", 4097)})
			if !errors.Is(err, artifact.ErrTooLarge) {
				t.Fatalf("expected size limit: %v", err)
			}
			entries, err := os.ReadDir(f.s.Store.Root)
			must(t, err)
			if len(entries) != 0 {
				t.Fatal("failed save left files")
			}
			var count int64
			must(t, f.s.DB.Model(&db.Artifact{}).Count(&count).Error)
			if count != 0 {
				t.Fatal("failed save left metadata")
			}
			preview, err := f.s.PreviewZIP(ctx, f.author, f.revision.ID, bytes.NewReader(zipBytes(t)))
			must(t, err)
			// Force mid-import quota failure; preview survives, imported tests do not.
			var record db.ImportPreview
			must(t, f.s.DB.First(&record, preview.ID).Error)
			var archive db.Artifact
			must(t, f.s.DB.First(&archive, record.ArtifactID).Error)
			f.s.Store.MaxTotalBytes = archive.SizeBytes + 10
			if err := f.s.CommitZIP(ctx, f.author, f.revision.ID, preview.ID); !errors.Is(err, artifact.ErrQuota) {
				t.Fatalf("expected quota failure: %v", err)
			}
			tests, err := db.ListTestCases(f.s.DB, f.revision.ID)
			must(t, err)
			if len(tests) != 0 {
				t.Fatal("partial ZIP import persisted")
			}
			entries, err = os.ReadDir(f.s.Store.Root)
			must(t, err)
			if len(entries) != 1 {
				t.Fatal("failed import left files")
			}
			f.s.Store.MaxTotalBytes = 1 << 20
			var wg sync.WaitGroup
			errs := make(chan error, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); errs <- f.s.CommitZIP(ctx, f.author, f.revision.ID, preview.ID) }()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				must(t, err)
			}
			tests, err = db.ListTestCases(f.s.DB, f.revision.ID)
			must(t, err)
			if len(tests) != 2 {
				t.Fatal("retry imported duplicate tests")
			}
			expired, err := f.s.PreviewZIP(ctx, f.author, f.revision.ID, bytes.NewReader(zipBytes(t)))
			must(t, err)
			f.now = f.now.Add(time.Hour)
			if err := f.s.CommitZIP(ctx, f.author, f.revision.ID, expired.ID); !errors.Is(err, content.ErrConflict) {
				t.Fatal("expired preview accepted")
			}
			// Unreferenced artifacts are collectible, referenced samples stay present.
			old, err := f.s.Store.Save(f.s.DB, f.author.ID, "orphan.txt", "text/plain", strings.NewReader("orphan"))
			must(t, err)
			past := f.now.Add(-3 * time.Hour)
			must(t, f.s.DB.Model(old).Update("created_at", past).Error)
			must(t, os.Chtimes(filepath.Join(f.s.Store.Root, old.StorageKey), past, past))
			staging := filepath.Join(f.s.Store.Root, ".upload-crashed")
			must(t, os.WriteFile(staging, []byte("incomplete"), 0o600))
			must(t, os.Chtimes(staging, past, past))
			n, err := f.s.Collect(ctx, f.admin)
			must(t, err)
			if n < 2 {
				t.Fatal("orphan not collected")
			}
			if _, err := os.Stat(staging); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("crashed staging upload remains")
			}
			if _, err := f.s.Store.Open(old); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("orphan file remains")
			}
			var in db.Artifact
			must(t, f.s.DB.First(&in, tests[0].InputArtifactID).Error)
			file, err := f.s.Store.Open(&in)
			must(t, err)
			must(t, file.Close())
		})
	}
}

func TestCanceledRequestDoesNotWrite(t *testing.T) {
	f := setup(t, testutil.NewDatabase(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.s.SaveTest(ctx, f.author, f.revision.ID, 0, content.TestInput{Input: "x", Output: "y"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
}
