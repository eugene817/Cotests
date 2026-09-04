package server

import (
	"errors"
	"log"
	"net/http"
	"time"

	"cotests/internal/db"

	"gorm.io/gorm"
)

func (h *Handler) PublicContests(w http.ResponseWriter, r *http.Request) {
	contests, err := db.ListPublicContests(h.DB, time.Now().UTC())
	if err != nil {
		log.Printf("list public contests: %v", err)
		http.Error(w, "Could not load contests.", http.StatusInternalServerError)
		return
	}
	data := PageData{
		Title:    "Contests",
		User:     UserFromContext(r.Context()),
		Template: "public_contests",
		Contests: contests,
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) PublicContest(w http.ResponseWriter, r *http.Request) {
	contestID, err := routeID(r, "contestID")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contest, err := db.GetPublicContest(h.DB, contestID, time.Now().UTC())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("get public contest: %v", err)
		http.Error(w, "Could not load contest.", http.StatusInternalServerError)
		return
	}
	series, err := db.ListSeries(h.DB, contest.ID)
	if err != nil {
		log.Printf("list public series: %v", err)
		http.Error(w, "Could not load series.", http.StatusInternalServerError)
		return
	}
	data := PageData{
		Title:    contest.Title,
		User:     UserFromContext(r.Context()),
		Template: "public_contest",
		Contest:  contest,
		Series:   series,
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}
