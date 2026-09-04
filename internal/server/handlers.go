package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"cotests/internal/db"

	"gorm.io/gorm"
)

type Handler struct {
	DB            *gorm.DB
	Tpl           Template
	SecureCookies bool
}

type Template interface {
	ExecuteTemplate(wr io.Writer, name string, data any) error
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	SQLDB, err := h.DB.DB()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "db: %v", err)
		return
	}
	if err := SQLDB.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "db: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "Home",
		User:     UserFromContext(r.Context()),
		Template: "home",
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "Login",
		Mode:     "login",
		Action:   "/login",
		Template: "auth_form",
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "Register",
		Mode:     "register",
		Action:   "/register",
		Template: "auth_form",
	}
	h.render(w, "layout", h.withCSRFToken(w, r, data))
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	email, password, name, err := readAuthForm(w, r)
	if err != nil {
		h.renderAuthError(w, r, "register", err.Error(), http.StatusBadRequest)
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAuthError(w, r, "register", "Your form has expired. Please try again.", http.StatusForbidden)
		return
	}

	if err := validateCredentials(email, password); err != nil {
		h.renderAuthError(w, r, "register", err.Error(), http.StatusBadRequest)
		return
	}

	user, err := db.CreateUser(h.DB, email, password, name)
	if err != nil {
		if db.IsDuplicateError(err) {
			h.renderAuthError(w, r, "register", "This email is already taken", http.StatusConflict)
		} else {
			log.Printf("create user: %v", err)
			h.renderAuthError(w, r, "register", "Registration failed. Please try again.", http.StatusInternalServerError)
		}
		return
	}

	_, token, err := db.CreateSession(h.DB, user.ID)
	if err != nil {
		log.Printf("session create: %v", err)
		hxRedirect(w, r, "/login")
		return
	}

	h.setSessionCookie(w, token)
	hxRedirect(w, r, "/")
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	email, password, _, err := readAuthForm(w, r)
	if err != nil {
		h.renderAuthError(w, r, "login", err.Error(), http.StatusBadRequest)
		return
	}
	if !h.validCSRFToken(r) {
		h.renderAuthError(w, r, "login", "Your form has expired. Please try again.", http.StatusForbidden)
		return
	}

	if err := validateCredentials(email, password); err != nil {
		h.renderAuthError(w, r, "login", "Invalid email or password", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserByEmail(h.DB, email)
	if err != nil {
		h.renderAuthError(w, r, "login", "Invalid email or password", http.StatusUnauthorized)
		return
	}
	if !user.CheckPassword(password) {
		h.renderAuthError(w, r, "login", "Invalid email or password", http.StatusUnauthorized)
		return
	}

	_, token, err := db.CreateSession(h.DB, user.ID)
	if err != nil {
		log.Printf("session create: %v", err)
		h.renderAuthError(w, r, "login", "Login failed. Please try again.", http.StatusInternalServerError)
		return
	}

	h.setSessionCookie(w, token)
	hxRedirect(w, r, "/")
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if !h.validCSRFToken(r) {
		renderAccessError(w, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
		return
	}
	cookie, err := r.Cookie("session_token")
	if err == nil {
		session, err := db.GetSessionByToken(h.DB, cookie.Value)
		if err == nil {
			if err := db.DeleteSession(h.DB, session.ID); err != nil {
				log.Printf("delete session: %v", err)
			}
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.SecureCookies,
	})
	hxRedirect(w, r, "/")
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(7 * 24 * time.Hour / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.SecureCookies,
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

func (h *Handler) renderAuthError(w http.ResponseWriter, r *http.Request, mode, message string, status int) {
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
	if isHTMX(r) {
		name = "auth_form_content"
	}
	data = h.withCSRFToken(w, r, data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	h.render(w, name, data)
}

func (h *Handler) withCSRFToken(w http.ResponseWriter, r *http.Request, data PageData) PageData {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		data.CSRFToken = cookie.Value
		return data
	}
	token, err := generateCSRFToken()
	if err != nil {
		log.Printf("generate csrf token: %v", err)
		return data
	}
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: token, Path: "/", MaxAge: int(7 * 24 * time.Hour / time.Second), SameSite: http.SameSiteLaxMode, Secure: h.SecureCookies})
	data.CSRFToken = token
	return data
}

func (h *Handler) validCSRFToken(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue("csrf_token")
	}
	return constantTimeEqual(cookie.Value, token)
}

func readAuthForm(w http.ResponseWriter, r *http.Request) (email, password, name string, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		return "", "", "", fmt.Errorf("invalid form submission")
	}
	return strings.ToLower(strings.TrimSpace(r.Form.Get("email"))), r.Form.Get("password"), strings.TrimSpace(r.Form.Get("name")), nil
}

func validateCredentials(email, password string) error {
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return fmt.Errorf("enter a valid email address")
	}
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("password must be between 8 and 72 characters")
	}
	return nil
}

func hxRedirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}
