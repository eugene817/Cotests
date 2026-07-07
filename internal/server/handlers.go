package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"cotests/internal/db"

	"gorm.io/gorm"
)

type Handler struct {
	DB    *gorm.DB
	SQLDB *sql.DB
	Tpl   Template
}

type Template interface {
	ExecuteTemplate(wr io.Writer, name string, data any) error
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.SQLDB.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "db: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	user := h.getUser(r)
	data := PageData{
		Title:    "Home",
		User:     user,
		Template: "home",
	}
	h.render(w, "layout", data)
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "Login",
		Mode:     "login",
		Action:   "/login",
		Template: "auth_form",
	}
	h.render(w, "layout", data)
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "Register",
		Mode:     "register",
		Action:   "/register",
		Template: "auth_form",
	}
	h.render(w, "layout", data)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	name := strings.TrimSpace(r.FormValue("name"))

	if email == "" || password == "" {
		h.renderAuthError(w, r, "register", "Email and password are required")
		return
	}

	user, err := db.CreateUser(h.DB, email, password, name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			h.renderAuthError(w, r, "register", "This email is already taken")
		} else {
			h.renderAuthError(w, r, "register", "Registration failed. Please try again.")
		}
		return
	}

	session, err := db.CreateSession(h.DB, user.ID)
	if err != nil {
		log.Printf("session create: %v", err)
		hxRedirect(w, r, "/login")
		return
	}

	h.setSessionCookie(w, session.Token)
	hxRedirect(w, r, "/")
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderAuthError(w, r, "login", "Email and password are required")
		return
	}

	user, err := db.GetUserByEmail(h.DB, email)
	if err != nil {
		h.renderAuthError(w, r, "login", "Invalid email or password")
		return
	}
	if !user.CheckPassword(password) {
		h.renderAuthError(w, r, "login", "Invalid email or password")
		return
	}

	session, err := db.CreateSession(h.DB, user.ID)
	if err != nil {
		h.renderAuthError(w, r, "login", "Login failed. Please try again.")
		return
	}

	h.setSessionCookie(w, session.Token)
	hxRedirect(w, r, "/")
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		session, err := db.GetSessionByToken(h.DB, cookie.Value)
		if err == nil {
			db.DeleteSession(h.DB, session.ID)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) getUser(r *http.Request) *db.User {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return nil
	}

	session, err := db.GetSessionByToken(h.DB, cookie.Value)
	if err != nil {
		return nil
	}
	if time.Now().After(session.ExpiresAt) {
		return nil
	}

	var user db.User
	if err := h.DB.First(&user, session.UserID).Error; err != nil {
		return nil
	}
	return &user
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(7 * 24 * time.Hour / time.Second),
		HttpOnly: true,
	})
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	if name == "layout" {
		if pd, ok := data.(PageData); ok && pd.Template != "" {
			var buf bytes.Buffer
			if err := h.Tpl.ExecuteTemplate(&buf, pd.Template, data); err != nil {
				log.Printf("template %s: %v", pd.Template, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			pd.Content = template.HTML(buf.String())
			data = pd
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s: %v", name, err)
	}
}

func (h *Handler) renderAuthError(w http.ResponseWriter, r *http.Request, mode, message string) {
	title := "Login"
	if mode == "register" {
		title = "Register"
	}
	data := PageData{
		Title:    title,
		Mode:     mode,
		Action:   "/" + mode,
		Error:    message,
		Template: "auth_form",
	}
	name := "layout"
	if r.Header.Get("HX-Request") == "true" {
		name = "auth_form"
	}
	h.render(w, name, data)
}

func hxRedirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}
