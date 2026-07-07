package server

import (
	"cotests/internal/db"
	"html/template"
)

type PageData struct {
	Title    string
	User     *db.User
	Error    string
	Mode     string
	Action   string
	Template string
	Content  template.HTML
}
