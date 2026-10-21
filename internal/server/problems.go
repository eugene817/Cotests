package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"cotests/internal/content"
	"cotests/internal/db"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
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
		h.renderAdminError(w, r, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	user := UserFromContext(r.Context())
	problem := &db.Problem{OwnerID: user.ID, Title: r.Form.Get("title"), Code: r.Form.Get("code")}
	revision := &db.ProblemRevision{Statement: r.Form.Get("statement"), LanguageMetadata: r.Form.Get("language_metadata"), CheckerMetadata: r.Form.Get("checker_metadata")}
	if err := db.CreateProblem(h.DB.WithContext(r.Context()), problem, revision); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d", problem.ID))
}

func (h *Handler) AdminProblem(w http.ResponseWriter, r *http.Request) {
	data, err := h.loadProblemPage(r)
	if err != nil {
		renderProblemLoadError(w, r, err)
		return
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) CreateProblemRevision(w http.ResponseWriter, r *http.Request) {
	problemID, err := routeID(r, "problemID")
	if err != nil {
		renderProblemLoadError(w, r, err)
		return
	}
	if _, err := db.CreateDraftRevision(h.DB.WithContext(r.Context()), problemID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.NotFound(w, r)
			return
		}
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d", problemID))
}

func (h *Handler) UpdateProblemRevision(w http.ResponseWriter, r *http.Request) {
	revision, err := h.loadProblemRevision(r)
	if err != nil {
		renderProblemLoadError(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	revision.Statement = r.Form.Get("statement")
	revision.LanguageMetadata = r.Form.Get("language_metadata")
	revision.CheckerMetadata = r.Form.Get("checker_metadata")
	if err := db.UpdateDraftRevision(h.DB.WithContext(r.Context()), revision); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d?revision=%d", revision.ProblemID, revision.ID))
}

func (h *Handler) PublishProblemRevision(w http.ResponseWriter, r *http.Request) {
	revision, err := h.loadProblemRevision(r)
	if err != nil {
		renderProblemLoadError(w, r, err)
		return
	}
	if err := db.PublishRevision(h.DB.WithContext(r.Context()), revision.ID, time.Now()); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d?revision=%d", revision.ProblemID, revision.ID))
}

func (h *Handler) CreateProblemTest(w http.ResponseWriter, r *http.Request) {
	revision, err := h.loadProblemRevision(r)
	if err != nil {
		renderProblemLoadError(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	position, err := strconv.Atoi(r.Form.Get("position"))
	if err != nil || position < 0 {
		h.renderAdminError(w, r, http.StatusBadRequest, "Test position must be a non-negative whole number.")
		return
	}
	user := UserFromContext(r.Context())
	if _, err := h.Content.SaveTest(r.Context(), user, revision.ID, 0, content.TestInput{Position: position, IsSample: r.Form.Get("is_sample") == "on", Input: r.Form.Get("input"), Output: r.Form.Get("output")}); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/problems/%d?revision=%d", revision.ProblemID, revision.ID))
}

func (h *Handler) loadProblemRevision(r *http.Request) (*db.ProblemRevision, error) {
	problemID, err := routeID(r, "problemID")
	if err != nil {
		return nil, err
	}
	query := h.DB.WithContext(r.Context()).Where("problem_id = ?", problemID)
	raw := chi.URLParam(r, "revisionID")
	if raw == "" {
		raw = r.URL.Query().Get("revision")
	}
	if raw != "" {
		id, err := parseID(raw)
		if err != nil {
			return nil, err
		}
		query = query.Where("id = ?", id)
	}
	var revision db.ProblemRevision
	if err := query.Order("number DESC").First(&revision).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func (h *Handler) loadProblemPage(r *http.Request) (PageData, error) {
	revision, err := h.loadProblemRevision(r)
	if err != nil {
		return PageData{}, err
	}
	database := h.DB.WithContext(r.Context())
	problem, err := db.GetProblem(database, revision.ProblemID)
	if err != nil {
		return PageData{}, err
	}
	revisions, err := db.ListProblemRevisions(database, problem.ID)
	if err != nil {
		return PageData{}, err
	}
	tests, err := db.ListTestCases(database, revision.ID)
	if err != nil {
		return PageData{}, err
	}
	return PageData{Title: problem.Title, User: UserFromContext(r.Context()), Template: "problem_detail", Problem: problem, Revision: revision, Tests: tests, Revisions: revisions}, nil
}

func renderProblemLoadError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.NotFound(w, r)
		return
	}
	log.Printf("load problem: %v", err)
	http.Error(w, "Could not load problem.", http.StatusInternalServerError)
}
