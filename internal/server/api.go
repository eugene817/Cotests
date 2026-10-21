package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cotests/internal/artifact"
	"cotests/internal/content"
	"cotests/internal/db"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type apiHandler struct {
	service *content.Service
	pages   *Handler
}
type apiOperation func(http.ResponseWriter, *http.Request) (any, error)

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func apiError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal", "Internal server error"
	var validation content.ValidationError
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, content.ErrUnauthorized):
		status, code, message = 401, "unauthorized", err.Error()
	case errors.Is(err, content.ErrForbidden):
		status, code, message = 403, "forbidden", "Access denied"
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, code, message = 404, "not_found", "Resource not found"
	case errors.Is(err, content.ErrConflict), errors.Is(err, db.ErrNotDraft), errors.Is(err, artifact.ErrQuota), db.IsDuplicateError(err):
		status, code, message = 409, "conflict", "Request conflicts with current state"
	case errors.As(err, &validation):
		status, code, message = 400, "validation", validation.Error()
	case errors.As(err, &tooLarge), errors.Is(err, artifact.ErrTooLarge):
		status, code, message = 413, "too_large", "Request exceeds size limit"
	default:
		log.Printf("content API: %v", err)
	}
	jsonResponse(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func endpoint(operation apiOperation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := operation(w, r)
		if err != nil {
			apiError(w, err)
			return
		}
		if value == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		jsonResponse(w, http.StatusOK, value)
	}
}

func decodeJSON(r *http.Request, value any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return content.ValidationError{Message: "Content-Type must be application/json"}
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return content.ValidationError{Message: "Invalid JSON body: " + err.Error()}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return content.ValidationError{Message: "Expected one JSON value"}
	}
	return nil
}

func apiID(r *http.Request, name string) (uint, error) {
	id, err := strconv.ParseUint(chi.URLParam(r, name), 10, 32)
	if err != nil || id == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return uint(id), nil
}

func pagination(r *http.Request, database *gorm.DB) (*gorm.DB, error) {
	limit, offset := 50, 0
	for key, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
		if raw := r.URL.Query().Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				return nil, content.ValidationError{Message: "Invalid pagination"}
			}
			*dest = value
		}
	}
	if limit < 1 || limit > 100 {
		return nil, content.ValidationError{Message: "limit must be between 1 and 100"}
	}
	return database.Limit(limit).Offset(offset), nil
}

func apiList[T any](r *http.Request, query *gorm.DB, order string) (any, error) {
	query, err := pagination(r, query)
	if err != nil {
		return nil, err
	}
	items := []T{}
	err = query.Order(order).Find(&items).Error
	return items, err
}

func (a *apiHandler) routes(r chi.Router) {
	r.Get("/csrf", endpoint(func(w http.ResponseWriter, r *http.Request) (any, error) {
		data := a.pages.withCSRFToken(w, r, PageData{})
		return map[string]string{"csrf_token": data.CSRFToken}, nil
	}))
	r.Get("/contests", endpoint(a.contests))
	r.Post("/contests", endpoint(a.createContest))
	r.Get("/contests/{id}", endpoint(a.contest))
	r.Put("/contests/{id}", endpoint(a.updateContest))
	r.Delete("/contests/{id}", endpoint(a.deleteContest))
	r.Get("/contests/{id}/members", endpoint(a.members))
	r.Put("/contests/{id}/members", endpoint(a.setMember))
	r.Delete("/contests/{id}/members/{userID}", endpoint(a.deleteMember))
	r.Get("/contests/{id}/series", endpoint(a.listSeries))
	r.Post("/contests/{id}/series", endpoint(a.saveSeries))
	r.Put("/contests/{id}/series/order", endpoint(a.orderSeries))
	r.Put("/series/{id}", endpoint(a.saveSeries))
	r.Delete("/series/{id}", endpoint(a.deleteSeries))
	r.Get("/series/{id}/problems", endpoint(a.assignments))
	r.Post("/series/{id}/problems", endpoint(a.assign))
	r.Put("/series/{id}/problems/order", endpoint(a.orderProblems))
	r.Delete("/series/{id}/problems/{assignmentID}", endpoint(a.unassign))
	r.Get("/problems", endpoint(a.problems))
	r.Post("/problems", endpoint(a.createProblem))
	r.Get("/problems/{id}", endpoint(a.problem))
	r.Put("/problems/{id}", endpoint(a.updateProblem))
	r.Get("/problems/{id}/revisions", endpoint(a.revisions))
	r.Post("/problems/{id}/revisions", endpoint(a.newRevision))
	r.Get("/revisions/{id}", endpoint(a.revision))
	r.Put("/revisions/{id}", endpoint(a.updateRevision))
	r.Post("/revisions/{id}/publish", endpoint(a.publish))
	r.Get("/revisions/{id}/tests", endpoint(a.tests))
	r.Post("/revisions/{id}/tests", endpoint(a.saveTest))
	r.Put("/revisions/{id}/tests/{testID}", endpoint(a.saveTest))
	r.Delete("/revisions/{id}/tests/{testID}", endpoint(a.deleteTest))
	r.Post("/revisions/{id}/pdf", endpoint(a.uploadPDF))
	r.Post("/revisions/{id}/imports", endpoint(a.previewZIP))
	r.Post("/revisions/{id}/imports/{previewID}/commit", endpoint(a.commitZIP))
	r.Get("/assignments/{id}", endpoint(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := apiID(r, "id")
		if err != nil {
			return nil, err
		}
		return a.service.Statement(r.Context(), UserFromContext(r.Context()), id)
	}))
	r.Get("/artifacts/{id}", a.download)
	r.Post("/artifacts/gc", endpoint(func(w http.ResponseWriter, r *http.Request) (any, error) {
		n, err := a.service.Collect(r.Context(), UserFromContext(r.Context()))
		return map[string]int{"removed": n}, err
	}))
}

func (a *apiHandler) database(r *http.Request) *gorm.DB { return a.service.DB.WithContext(r.Context()) }
func (a *apiHandler) manager(r *http.Request) (uint, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return 0, err
	}
	return id, a.service.ManageContest(r.Context(), UserFromContext(r.Context()), id)
}
func apiAdmin(r *http.Request) error {
	user := UserFromContext(r.Context())
	if user == nil {
		return content.ErrUnauthorized
	}
	if user.Role != db.RoleAdmin {
		return content.ErrForbidden
	}
	return nil
}

type contestInput struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Visibility  string     `json:"visibility"`
	StartAt     *time.Time `json:"start_at"`
	EndAt       *time.Time `json:"end_at"`
}

func (i contestInput) model() *db.Contest {
	return &db.Contest{Title: i.Title, Description: i.Description, Visibility: i.Visibility, StartAt: i.StartAt, EndAt: i.EndAt}
}
func (a *apiHandler) contests(w http.ResponseWriter, r *http.Request) (any, error) {
	query := a.database(r).Model(&db.Contest{})
	actor := UserFromContext(r.Context())
	if actor == nil || actor.Role != db.RoleAdmin {
		if actor == nil {
			query = query.Where("visibility = ?", db.ContestPublished)
		} else {
			query = query.Where("visibility = ? OR id IN (SELECT contest_id FROM contest_memberships WHERE user_id = ? AND role = ?)", db.ContestPublished, actor.ID, "organizer")
		}
	}
	return apiList[db.Contest](r, query, "id DESC")
}
func (a *apiHandler) createContest(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := apiAdmin(r); err != nil {
		return nil, err
	}
	var input contestInput
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	item := input.model()
	return item, db.CreateContest(a.database(r), item)
}
func (a *apiHandler) contest(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	item, err := db.GetContest(a.database(r), id)
	if err != nil {
		return nil, err
	}
	if item.Visibility != db.ContestPublished {
		if err := a.service.ManageContest(r.Context(), UserFromContext(r.Context()), id); err != nil {
			return nil, err
		}
	}
	return item, nil
}
func (a *apiHandler) updateContest(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := a.manager(r)
	if err != nil {
		return nil, err
	}
	var input contestInput
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	item := input.model()
	item.ID = id
	return item, db.UpdateContest(a.database(r), item)
}
func (a *apiHandler) deleteContest(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := apiAdmin(r); err != nil {
		return nil, err
	}
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, db.DeleteContest(a.database(r), id)
}
func (a *apiHandler) members(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := a.manager(r)
	if err != nil {
		return nil, err
	}
	return apiList[db.ContestMembership](r, a.database(r).Where("contest_id = ?", id), "id")
}
func (a *apiHandler) setMember(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	var input struct {
		UserID uint   `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	return nil, a.service.SetMember(r.Context(), UserFromContext(r.Context()), id, input.UserID, input.Role, false)
}
func (a *apiHandler) deleteMember(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	uid, err := apiID(r, "userID")
	if err != nil {
		return nil, err
	}
	return nil, a.service.SetMember(r.Context(), UserFromContext(r.Context()), id, uid, "", true)
}
func (a *apiHandler) listSeries(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.contest(w, r); err != nil {
		return nil, err
	}
	id, _ := apiID(r, "id")
	return apiList[db.Series](r, a.database(r).Where("contest_id = ?", id), "position")
}
func (a *apiHandler) saveSeries(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	item := &db.Series{ContestID: id}
	if r.Method == http.MethodPut {
		item, err = a.service.Series(r.Context(), UserFromContext(r.Context()), id)
		if err != nil {
			return nil, err
		}
	}
	var input struct {
		Title     string     `json:"title"`
		Position  int        `json:"position"`
		StartAt   *time.Time `json:"start_at"`
		EndAt     *time.Time `json:"end_at"`
		ReleaseAt *time.Time `json:"release_at"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	item.Title = input.Title
	item.Position = input.Position
	item.StartAt = input.StartAt
	item.EndAt = input.EndAt
	item.ReleaseAt = input.ReleaseAt
	return item, a.service.SaveSeries(r.Context(), UserFromContext(r.Context()), item)
}
func (a *apiHandler) deleteSeries(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	if _, err := a.service.Series(r.Context(), UserFromContext(r.Context()), id); err != nil {
		return nil, err
	}
	return nil, db.DeleteSeries(a.database(r), id)
}
func (a *apiHandler) order(r *http.Request, problems bool) error {
	id, err := apiID(r, "id")
	if err != nil {
		return err
	}
	var input struct {
		IDs []uint `json:"ids"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	return a.service.Reorder(r.Context(), UserFromContext(r.Context()), id, input.IDs, problems)
}
func (a *apiHandler) orderSeries(w http.ResponseWriter, r *http.Request) (any, error) {
	return nil, a.order(r, false)
}
func (a *apiHandler) orderProblems(w http.ResponseWriter, r *http.Request) (any, error) {
	return nil, a.order(r, true)
}
func (a *apiHandler) assignments(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	series, err := db.GetSeries(a.database(r), id)
	if err != nil {
		return nil, err
	}
	if err := a.service.CanReadSeries(r.Context(), UserFromContext(r.Context()), series); err != nil {
		return nil, err
	}
	return apiList[db.SeriesProblem](r, a.database(r).Where("series_id = ?", id), "position")
}
func (a *apiHandler) assign(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	var input struct {
		RevisionID uint   `json:"revision_id"`
		Code       string `json:"code"`
		Position   int    `json:"position"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	item := &db.SeriesProblem{SeriesID: id, PublishedRevisionID: input.RevisionID, Code: input.Code, Position: input.Position}
	return item, a.service.Assign(r.Context(), UserFromContext(r.Context()), item)
}
func (a *apiHandler) unassign(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	if _, err := a.service.Series(r.Context(), UserFromContext(r.Context()), id); err != nil {
		return nil, err
	}
	aid, err := apiID(r, "assignmentID")
	if err != nil {
		return nil, err
	}
	result := a.database(r).Where("id = ? AND series_id = ?", aid, id).Delete(&db.SeriesProblem{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return nil, nil
}

func (a *apiHandler) problems(w http.ResponseWriter, r *http.Request) (any, error) {
	actor := UserFromContext(r.Context())
	if err := a.service.CanAuthor(r.Context(), actor); err != nil {
		return nil, err
	}
	q := a.database(r).Model(&db.Problem{})
	if actor.Role != db.RoleAdmin {
		q = q.Where("owner_id = ?", actor.ID)
	}
	return apiList[db.Problem](r, q, "id DESC")
}

type problemInput struct {
	Title     string   `json:"title"`
	Code      string   `json:"code"`
	Statement string   `json:"statement"`
	Languages []string `json:"languages"`
	Checker   string   `json:"checker"`
}

func (a *apiHandler) createProblem(w http.ResponseWriter, r *http.Request) (any, error) {
	actor := UserFromContext(r.Context())
	if err := a.service.CanAuthor(r.Context(), actor); err != nil {
		return nil, err
	}
	var input problemInput
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	languages := "[]"
	if input.Languages != nil {
		raw, _ := json.Marshal(input.Languages)
		languages = string(raw)
	}
	problem := &db.Problem{OwnerID: actor.ID, Title: input.Title, Code: input.Code}
	revision := &db.ProblemRevision{Statement: input.Statement, LanguageMetadata: languages, CheckerMetadata: input.Checker}
	if err := db.CreateProblem(a.database(r), problem, revision); err != nil {
		return nil, err
	}
	return map[string]any{"problem": problem, "revision": revision}, nil
}
func (a *apiHandler) problem(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	return a.service.ManageProblem(r.Context(), UserFromContext(r.Context()), id)
}
func (a *apiHandler) updateProblem(w http.ResponseWriter, r *http.Request) (any, error) {
	value, err := a.problem(w, r)
	if err != nil {
		return nil, err
	}
	item := value.(*db.Problem)
	var input struct {
		Title string `json:"title"`
		Code  string `json:"code"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Code = strings.TrimSpace(input.Code)
	if input.Title == "" || input.Code == "" || len(input.Title) > 300 || len(input.Code) > 64 {
		return nil, content.ValidationError{Message: "Invalid title or code"}
	}
	err = a.database(r).Model(item).Updates(map[string]any{"title": input.Title, "code": input.Code}).Error
	item.Title = input.Title
	item.Code = input.Code
	return item, err
}
func (a *apiHandler) revisions(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.problem(w, r); err != nil {
		return nil, err
	}
	id, _ := apiID(r, "id")
	return apiList[db.ProblemRevision](r, a.database(r).Where("problem_id = ?", id), "number DESC")
}
func (a *apiHandler) newRevision(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.problem(w, r); err != nil {
		return nil, err
	}
	id, _ := apiID(r, "id")
	return db.CreateDraftRevision(a.database(r), id)
}
func (a *apiHandler) revision(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	return a.service.Revision(r.Context(), UserFromContext(r.Context()), id)
}
func (a *apiHandler) updateRevision(w http.ResponseWriter, r *http.Request) (any, error) {
	value, err := a.revision(w, r)
	if err != nil {
		return nil, err
	}
	item := value.(*db.ProblemRevision)
	if item.State != db.RevisionDraft {
		return nil, content.ErrConflict
	}
	var input struct {
		Statement       string   `json:"statement"`
		Languages       []string `json:"languages"`
		Checker         string   `json:"checker"`
		TimeLimitMillis *int64   `json:"time_limit_millis"`
		MemoryLimitKiB  *int64   `json:"memory_limit_kib"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	if input.Languages == nil {
		input.Languages = []string{}
	}
	raw, _ := json.Marshal(input.Languages)
	item.Statement = input.Statement
	item.LanguageMetadata = string(raw)
	item.CheckerMetadata = input.Checker
	item.TimeLimitMillis = input.TimeLimitMillis
	item.MemoryLimitKiB = input.MemoryLimitKiB
	return item, db.UpdateDraftRevision(a.database(r), item)
}
func (a *apiHandler) publish(w http.ResponseWriter, r *http.Request) (any, error) {
	value, err := a.revision(w, r)
	if err != nil {
		return nil, err
	}
	item := value.(*db.ProblemRevision)
	if item.State != db.RevisionDraft {
		return nil, content.ErrConflict
	}
	return nil, db.PublishRevision(a.database(r), item.ID, time.Now())
}
func (a *apiHandler) tests(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.revision(w, r); err != nil {
		return nil, err
	}
	id, _ := apiID(r, "id")
	return apiList[db.TestCase](r, a.database(r).Where("problem_revision_id = ?", id), "position")
}
func (a *apiHandler) saveTest(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	var tid uint
	if r.Method == http.MethodPut {
		tid, err = apiID(r, "testID")
		if err != nil {
			return nil, err
		}
	}
	var input content.TestInput
	if err := decodeJSON(r, &input); err != nil {
		return nil, err
	}
	return a.service.SaveTest(r.Context(), UserFromContext(r.Context()), id, tid, input)
}
func (a *apiHandler) deleteTest(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	tid, err := apiID(r, "testID")
	if err != nil {
		return nil, err
	}
	return nil, a.service.DeleteTest(r.Context(), UserFromContext(r.Context()), id, tid)
}

func upload(r *http.Request, fn func(io.Reader) (any, error)) (any, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, content.ValidationError{Message: "Expected multipart upload"}
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" {
		return nil, content.ValidationError{Message: "Expected file field"}
	}
	defer part.Close()
	// Validate the entire envelope before committing effects; the reader is bounded
	// by route middleware. Reject duplicate/extra fields deterministically.
	file, err := os.CreateTemp("", "cotests-upload-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := io.Copy(file, part); err != nil {
		return nil, err
	}
	if _, err := reader.NextPart(); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, content.ValidationError{Message: "Only one file is allowed"}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return fn(file)
}
func (a *apiHandler) uploadPDF(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	if _, err := a.service.Revision(r.Context(), UserFromContext(r.Context()), id); err != nil {
		return nil, err
	}
	return upload(r, func(source io.Reader) (any, error) {
		return a.service.UploadPDF(r.Context(), UserFromContext(r.Context()), id, source)
	})
}
func (a *apiHandler) previewZIP(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	if _, err := a.service.Revision(r.Context(), UserFromContext(r.Context()), id); err != nil {
		return nil, err
	}
	return upload(r, func(source io.Reader) (any, error) {
		return a.service.PreviewZIP(r.Context(), UserFromContext(r.Context()), id, source)
	})
}
func (a *apiHandler) commitZIP(w http.ResponseWriter, r *http.Request) (any, error) {
	id, err := apiID(r, "id")
	if err != nil {
		return nil, err
	}
	pid, err := apiID(r, "previewID")
	if err != nil {
		return nil, err
	}
	return nil, a.service.CommitZIP(r.Context(), UserFromContext(r.Context()), id, pid)
}
func (a *apiHandler) download(w http.ResponseWriter, r *http.Request) {
	id, err := apiID(r, "id")
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := a.service.Artifact(r.Context(), UserFromContext(r.Context()), id)
	if err != nil {
		apiError(w, err)
		return
	}
	file, err := a.service.Store.Open(item)
	if err != nil {
		apiError(w, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, item.Filename, item.CreatedAt, file)
}
