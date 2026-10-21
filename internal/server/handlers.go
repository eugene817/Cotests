package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"cotests/internal/auth"
	"cotests/internal/content"
	"cotests/internal/db"

	"gorm.io/gorm"
)

type Handler struct {
	DB               *gorm.DB
	Tpl              Template
	SecureCookies    bool
	ArtifactDir      string
	MaxArtifactBytes int64
	Content          *content.Service
	CSRFKey          []byte
	authWork         chan struct{}
	authAttempts     map[string]authAttempt
	authAttemptsMu   sync.Mutex
}

type Template interface {
	ExecuteTemplate(wr io.Writer, name string, data any) error
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	SQLDB, err := h.DB.DB()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Printf("readiness: %v", err)
		w.Write([]byte("not ready"))
		return
	}
	if err := SQLDB.PingContext(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Printf("readiness: %v", err)
		w.Write([]byte("not ready"))
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
	select {
	case h.authWork <- struct{}{}:
		defer func() { <-h.authWork }()
	default:
		h.renderAuthError(w, r, "register", "Please wait before trying again.", http.StatusTooManyRequests)
		return
	}
	email, password, name, err := readAuthForm(w, r)
	if err != nil {
		h.renderAuthError(w, r, "register", err.Error(), http.StatusBadRequest)
		return
	}
	if !h.allowAuthAttempt(r, email) {
		h.renderAuthError(w, r, "register", "Please wait before trying again.", http.StatusTooManyRequests)
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

	user, err := db.CreateUser(h.DB.WithContext(r.Context()), email, password, name)
	if err != nil {
		if db.IsDuplicateError(err) {
			h.renderAuthError(w, r, "register", "This email is already taken", http.StatusConflict)
		} else {
			log.Printf("create user: %v", err)
			h.renderAuthError(w, r, "register", "Registration failed. Please try again.", http.StatusInternalServerError)
		}
		return
	}

	_, token, err := db.CreateSession(h.DB.WithContext(r.Context()), user.ID)
	if err != nil {
		log.Printf("session create: %v", err)
		hxRedirect(w, r, "/login")
		return
	}

	h.setSessionCookie(w, token)
	hxRedirect(w, r, "/")
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	select {
	case h.authWork <- struct{}{}:
		defer func() { <-h.authWork }()
	default:
		h.renderAuthError(w, r, "login", "Please wait before trying again.", http.StatusTooManyRequests)
		return
	}
	email, password, _, err := readAuthForm(w, r)
	if err != nil {
		h.renderAuthError(w, r, "login", err.Error(), http.StatusBadRequest)
		return
	}
	if !h.allowAuthAttempt(r, email) {
		h.renderAuthError(w, r, "login", "Please wait before trying again.", http.StatusTooManyRequests)
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

	user, err := db.GetUserByEmail(h.DB.WithContext(r.Context()), email)
	if err != nil {
		h.renderAuthError(w, r, "login", "Invalid email or password", http.StatusUnauthorized)
		return
	}
	if !user.CheckPassword(password) {
		h.renderAuthError(w, r, "login", "Invalid email or password", http.StatusUnauthorized)
		return
	}

	_, token, err := db.CreateSession(h.DB.WithContext(r.Context()), user.ID)
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
		session, err := db.GetSessionByToken(h.DB.WithContext(r.Context()), cookie.Value)
		if err == nil {
			if err := db.DeleteSession(h.DB.WithContext(r.Context()), session.ID); err != nil {
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
	if cookie, err := r.Cookie(csrfCookieName); err == nil && h.validSignedCSRF(r, cookie.Value) {
		data.CSRFToken = cookie.Value
		return data
	}
	token, err := generateCSRFToken()
	if err != nil {
		log.Printf("generate csrf token: %v", err)
		return data
	}
	token = h.signCSRF(r, token)
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: token, Path: "/", MaxAge: int(7 * 24 * time.Hour / time.Second), SameSite: http.SameSiteLaxMode, Secure: h.SecureCookies})
	data.CSRFToken = token
	return data
}

func (h *Handler) validCSRFToken(r *http.Request) bool {
	return validRequestCSRF(r)
}

func readAuthForm(w http.ResponseWriter, r *http.Request) (email, password, name string, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		return "", "", "", fmt.Errorf("invalid form submission")
	}
	return strings.ToLower(strings.TrimSpace(r.Form.Get("email"))), r.Form.Get("password"), strings.TrimSpace(r.Form.Get("name")), nil
}

func validateCredentials(email, password string) error {
	if _, err := auth.NormalizeEmail(email); err != nil {
		return err
	}
	return auth.ValidatePassword(password)
}

func hxRedirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}

type authAttempt struct {
	count int
	start time.Time
}

// Bound costly bcrypt work by both source address and normalized email. The
// buckets expire quickly so a client cannot permanently lock out an account.
func (h *Handler) allowAuthAttempt(r *http.Request, email string) bool {
	const limit = 10
	const window = time.Minute
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	now := time.Now()
	h.authAttemptsMu.Lock()
	defer h.authAttemptsMu.Unlock()
	if h.authAttempts == nil {
		h.authAttempts = make(map[string]authAttempt)
	}
	for key, attempt := range h.authAttempts {
		if now.Sub(attempt.start) >= window {
			delete(h.authAttempts, key)
		}
	}
	if len(h.authAttempts) > 10000 {
		return false
	}
	keys := []string{"ip:" + host, "email:" + strings.ToLower(strings.TrimSpace(email))}
	for _, key := range keys {
		attempt := h.authAttempts[key]
		if attempt.start.IsZero() || now.Sub(attempt.start) >= window {
			attempt = authAttempt{start: now}
		}
		if attempt.count >= limit {
			return false
		}
	}
	for _, key := range keys {
		attempt := h.authAttempts[key]
		if attempt.start.IsZero() || now.Sub(attempt.start) >= window {
			attempt = authAttempt{start: now}
		}
		attempt.count++
		h.authAttempts[key] = attempt
	}
	return true
}
