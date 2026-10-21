package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cotests/internal/content"
	"cotests/internal/db"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) AdminProblems(w http.ResponseWriter, r *http.Request) {
	problems, err := db.ListProblems(h.DB.WithContext(r.Context()))
	if err != nil {
		http.Error(w, "Could not load problems.", http.StatusInternalServerError)
		return
	}
	h.render(w, "layout", h.withCSRFToken(w, r, PageData{Title: "Problems", User: UserFromContext(r.Context()), Template: "problems", Problems: problems}))
}

func (h *Handler) CreateProblem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	user := UserFromContext(r.Context())
	problem := &db.Problem{OwnerID: user.ID, Title: r.Form.Get("title"), Code: r.Form.Get("code")}
	revision := &db.ProblemRevision{Statement: r.Form.Get("statement"), LanguageMetadata: r.Form.Get("language_metadata"), CheckerMetadata: r.Form.Get("checker_metadata")}
	if err := db.CreateProblem(h.DB.WithContext(r.Context()), problem, revision); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d", problem.ID))
}

func (h *Handler) AdminProblem(w http.ResponseWriter, r *http.Request) {
	problem, revision, tests, revisions, ok := h.loadAdminProblem(w, r)
	if !ok {
		return
	}
	h.render(w, "layout", h.withCSRFToken(w, r, PageData{Title: problem.Title, User: UserFromContext(r.Context()), Template: "problem_detail", Problem: problem, Revision: revision, Tests: tests, Revisions: revisions}))
}

func (h *Handler) CreateProblemRevision(w http.ResponseWriter, r *http.Request) {
	problem, _, _, _, ok := h.loadAdminProblem(w, r)
	if !ok {
		return
	}
	if _, err := db.CreateDraftRevision(h.DB.WithContext(r.Context()), problem.ID); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d", problem.ID))
}

func (h *Handler) UpdateProblemRevision(w http.ResponseWriter, r *http.Request) {
	problem, revision, _, _, ok := h.loadAdminProblem(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	revision.Statement = r.Form.Get("statement")
	revision.LanguageMetadata = r.Form.Get("language_metadata")
	revision.CheckerMetadata = r.Form.Get("checker_metadata")
	if err := db.UpdateDraftRevision(h.DB.WithContext(r.Context()), revision); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d?revision=%d", problem.ID, revision.ID))
}

func (h *Handler) PublishProblemRevision(w http.ResponseWriter, r *http.Request) {
	problem, revision, _, _, ok := h.loadAdminProblem(w, r)
	if !ok {
		return
	}
	if err := db.PublishRevision(h.DB.WithContext(r.Context()), revision.ID, time.Now()); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d?revision=%d", problem.ID, revision.ID))
}

func (h *Handler) CreateProblemTest(w http.ResponseWriter, r *http.Request) {
	problem, revision, _, _, ok := h.loadAdminProblem(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	position, err := strconv.Atoi(r.Form.Get("position"))
	if err != nil || position < 0 {
		h.renderProblemError(w, r, http.StatusBadRequest, "Test position must be a non-negative whole number.")
		return
	}
	user := UserFromContext(r.Context())
	if _, err := h.Content.SaveTest(r.Context(), user, revision.ID, 0, content.TestInput{Position: position, IsSample: r.Form.Get("is_sample") == "on", Input: r.Form.Get("input"), Output: r.Form.Get("output")}); err != nil {
		h.renderProblemError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d?revision=%d", problem.ID, revision.ID))
}

func (h *Handler) loadAdminProblem(w http.ResponseWriter, r *http.Request) (*db.Problem, *db.ProblemRevision, []db.TestCase, []db.ProblemRevision, bool) {
	problemID, err := routeID(r, "problemID")
	if err != nil {
		http.NotFound(w, r)
		return nil, nil, nil, nil, false
	}
	problem, err := db.GetProblem(h.DB.WithContext(r.Context()), problemID)
	if err != nil {
		http.NotFound(w, r)
		return nil, nil, nil, nil, false
	}
	revisions, err := db.ListProblemRevisions(h.DB.WithContext(r.Context()), problem.ID)
	if err != nil || len(revisions) == 0 {
		http.Error(w, "Could not load problem revisions.", http.StatusInternalServerError)
		return nil, nil, nil, nil, false
	}
	revision := &revisions[0]
	raw := chi.URLParam(r, "revisionID")
	if raw == "" {
		raw = r.URL.Query().Get("revision")
	}
	if raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil, nil, nil, nil, false
		}
		candidate, err := db.GetProblemRevision(h.DB.WithContext(r.Context()), uint(id))
		if err != nil || candidate.ProblemID != problem.ID {
			http.NotFound(w, r)
			return nil, nil, nil, nil, false
		}
		revision = candidate
	}
	tests, err := db.ListTestCases(h.DB.WithContext(r.Context()), revision.ID)
	if err != nil {
		http.Error(w, "Could not load tests.", http.StatusInternalServerError)
		return nil, nil, nil, nil, false
	}
	return problem, revision, tests, revisions, true
}

func (h *Handler) renderProblemError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if isHTMX(r) {
		w.WriteHeader(status)
		h.render(w, "admin_error", PageData{Error: message})
		return
	}
	renderAccessError(w, status, message)
}
