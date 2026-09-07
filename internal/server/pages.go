package server

import (
	"cotests/internal/db"
	"html/template"
)

type PageData struct {
	Title     string
	User      *db.User
	Error     string
	Mode      string
	Action    string
	Template  string
	Content   template.HTML
	CSRFToken string
	Contest   *db.Contest
	Contests  []db.Contest
	Series    []db.Series
}
