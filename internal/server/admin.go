package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cotests/internal/db"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	contests, err := db.ListContests(h.DB)
	if err != nil {
		log.Printf("list contests: %v", err)
		http.Error(w, "Could not load contests.", http.StatusInternalServerError)
		return
	}
	data := PageData{
		Title:    "Contests",
		User:     UserFromContext(r.Context()),
		Template: "admin",
		Contest:  &db.Contest{Visibility: db.ContestDraft},
		Contests: contests,
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) AdminRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/contests", http.StatusMovedPermanently)
}

func (h *Handler) AdminContest(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.adminContest(w, r)
	if !ok {
		return
	}
	series, err := db.ListSeries(h.DB, contest.ID)
	if err != nil {
		log.Printf("list series: %v", err)
		http.Error(w, "Could not load series.", http.StatusInternalServerError)
		return
	}
	data := PageData{
		Title:    contest.Title,
		User:     UserFromContext(r.Context()),
		Template: "contest_detail",
		Contest:  contest,
		Series:   series,
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) CreateContest(w http.ResponseWriter, r *http.Request) {
	contest, err := readContestForm(w, r)
	if err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAdminError(w, r, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	if err := db.CreateContest(h.DB, contest); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if isHTMX(r) {
		w.Header().Set("HX-Retarget", "#contest-list")
		w.Header().Set("HX-Reswap", "afterbegin")
		h.render(w, "contest_card", contest)
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/contests/%d", contest.ID))
}

func (h *Handler) UpdateContest(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.adminContest(w, r)
	if !ok {
		return
	}
	input, err := readContestForm(w, r)
	if err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAdminError(w, r, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	input.ID = contest.ID
	if err := db.UpdateContest(h.DB, input); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/contests/%d", contest.ID))
}

func (h *Handler) DeleteContest(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.adminContest(w, r)
	if !ok {
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAdminError(w, r, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	if err := db.DeleteContest(h.DB, contest.ID); err != nil {
		log.Printf("delete contest: %v", err)
		http.Error(w, "Could not delete contest.", http.StatusInternalServerError)
		return
	}
	hxRedirect(w, r, "/admin/contests")
}

func (h *Handler) CreateSeries(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.adminContest(w, r)
	if !ok {
		return
	}
	series, err := readSeriesForm(w, r)
	if err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAdminError(w, r, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	series.ContestID = contest.ID
	if err := db.CreateSeries(h.DB, series); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/contests/%d", contest.ID))
}

func (h *Handler) UpdateSeries(w http.ResponseWriter, r *http.Request) {
	series, ok := h.adminSeries(w, r)
	if !ok {
		return
	}
	input, err := readSeriesForm(w, r)
	if err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAdminError(w, r, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	input.ID = series.ID
	if err := db.UpdateSeries(h.DB, input); err != nil {
		h.renderAdminError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/contests/%d", series.ContestID))
}

func (h *Handler) DeleteSeries(w http.ResponseWriter, r *http.Request) {
	series, ok := h.adminSeries(w, r)
	if !ok {
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAdminError(w, r, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	if err := db.DeleteSeries(h.DB, series.ID); err != nil {
		log.Printf("delete series: %v", err)
		http.Error(w, "Could not delete series.", http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.WriteHeader(http.StatusOK)
		return
	}
	hxRedirect(w, r, fmt.Sprintf("/admin/contests/%d", series.ContestID))
}

func (h *Handler) adminContest(w http.ResponseWriter, r *http.Request) (*db.Contest, bool) {
	contestID, err := routeID(r, "contestID")
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	contest, err := db.GetContest(h.DB, contestID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		log.Printf("get contest: %v", err)
		http.Error(w, "Could not load contest.", http.StatusInternalServerError)
		return nil, false
	}
	return contest, true
}

func (h *Handler) adminSeries(w http.ResponseWriter, r *http.Request) (*db.Series, bool) {
	contestID, err := routeID(r, "contestID")
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	seriesID, err := routeID(r, "seriesID")
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	series, err := db.GetSeries(h.DB, seriesID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		log.Printf("get series: %v", err)
		http.Error(w, "Could not load series.", http.StatusInternalServerError)
		return nil, false
	}
	if series.ContestID != contestID {
		http.NotFound(w, r)
		return nil, false
	}
	return series, true
}

func routeID(r *http.Request, key string) (uint, error) {
	id, err := strconv.ParseUint(chi.URLParam(r, key), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return uint(id), nil
}

func readContestForm(w http.ResponseWriter, r *http.Request) (*db.Contest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form submission")
	}
	startAt, err := parseDateTime(r.Form.Get("start_at"))
	if err != nil {
		return nil, fmt.Errorf("invalid start time")
	}
	endAt, err := parseDateTime(r.Form.Get("end_at"))
	if err != nil {
		return nil, fmt.Errorf("invalid end time")
	}
	return &db.Contest{
		Title:       r.Form.Get("title"),
		Description: r.Form.Get("description"),
		Visibility:  r.Form.Get("visibility"),
		StartAt:     startAt,
		EndAt:       endAt,
	}, nil
}

func readSeriesForm(w http.ResponseWriter, r *http.Request) (*db.Series, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form submission")
	}
	position, err := strconv.Atoi(r.Form.Get("position"))
	if err != nil {
		return nil, fmt.Errorf("position must be a whole number")
	}
	return &db.Series{Title: r.Form.Get("title"), Position: position}, nil
}

func parseDateTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04", value, time.UTC)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (h *Handler) renderAdminError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if !isHTMX(r) {
		renderAccessError(w, status, message)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	h.render(w, "admin_error", PageData{Error: message})
}
