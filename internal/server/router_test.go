package server

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"cotests/internal/db"
	"gorm.io/gorm"
)

func newTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tpl := template.Must(template.New("pages").Parse(`
{{define "layout"}}{{.Content}}{{end}}
{{define "home"}}home{{end}}
{{define "auth_form"}}{{.Error}}<form><input value="{{.CSRFToken}}"></form>{{end}}
{{define "admin"}}admin{{end}}`))
	return NewRouter(database, http.NotFoundHandler(), tpl, Config{SecureCookies: true}), database
}

func getCSRFCookie(t *testing.T, router http.Handler, path string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			return cookie
		}
	}
	t.Fatal("CSRF cookie not set")
	return nil
}

func TestRegisterSetsSecureSessionAndAdminRole(t *testing.T) {
	router, database := newTestRouter(t)
	csrf := getCSRFCookie(t, router, "/register")
	form := url.Values{"email": {"admin@example.com"}, "password": {"password1"}, "name": {"Admin"}, "csrf_token": {csrf.Value}}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrf)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("registration status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	var user db.User
	if err := database.First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.Role != db.RoleAdmin {
		t.Fatalf("role = %q, want %q", user.Role, db.RoleAdmin)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "session_token" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unsafe session cookie: %#v", sessionCookie)
	}
}

func TestRegisterRejectsMissingCSRFToken(t *testing.T) {
	router, _ := newTestRouter(t)
	form := url.Values{"email": {"admin@example.com"}, "password": {"password1"}}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	router, _ := newTestRouter(t)
	csrf := getCSRFCookie(t, router, "/register")
	form := url.Values{"email": {"admin@example.com"}, "password": {"password1"}, "csrf_token": {csrf.Value}}
	for attempt := 0; attempt < 2; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(csrf)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if attempt == 0 && response.Code != http.StatusSeeOther {
			t.Fatalf("first registration status = %d, want %d", response.Code, http.StatusSeeOther)
		}
		if attempt == 1 && response.Code != http.StatusConflict {
			t.Fatalf("duplicate registration status = %d, want %d", response.Code, http.StatusConflict)
		}
	}
}

func TestAdminAuthorization(t *testing.T) {
	router, database := newTestRouter(t)
	admin, err := db.CreateUser(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	member, err := db.CreateUser(database, "member@example.com", "password1", "Member")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	_, adminToken, err := db.CreateSession(database, admin.ID)
	if err != nil {
		t.Fatalf("create admin session: %v", err)
	}
	_, memberToken, err := db.CreateSession(database, member.ID)
	if err != nil {
		t.Fatalf("create member session: %v", err)
	}

	guest := httptest.NewRecorder()
	router.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if guest.Code != http.StatusSeeOther || guest.Result().Header.Get("Location") != "/login" {
		t.Fatalf("guest response = %d %q", guest.Code, guest.Result().Header.Get("Location"))
	}

	memberRequest := httptest.NewRequest(http.MethodGet, "/admin", nil)
	memberRequest.AddCookie(&http.Cookie{Name: "session_token", Value: memberToken})
	memberResponse := httptest.NewRecorder()
	router.ServeHTTP(memberResponse, memberRequest)
	if memberResponse.Code != http.StatusForbidden {
		t.Fatalf("member status = %d, want %d", memberResponse.Code, http.StatusForbidden)
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin", nil)
	adminRequest.AddCookie(&http.Cookie{Name: "session_token", Value: adminToken})
	adminResponse := httptest.NewRecorder()
	router.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK || !strings.Contains(adminResponse.Body.String(), "admin") {
		t.Fatalf("admin response = %d %q", adminResponse.Code, adminResponse.Body.String())
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	router, database := newTestRouter(t)
	user, err := db.CreateUser(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, token, err := db.CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	homeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRequest.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	homeResponse := httptest.NewRecorder()
	router.ServeHTTP(homeResponse, homeRequest)
	var csrf *http.Cookie
	for _, cookie := range homeResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrf = cookie
		}
	}
	if csrf == nil {
		t.Fatal("CSRF cookie not set on home page")
	}

	form := url.Values{"csrf_token": {csrf.Value}}
	logoutRequest := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(form.Encode()))
	logoutRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	logoutRequest.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	logoutRequest.AddCookie(csrf)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d, want %d", logoutResponse.Code, http.StatusSeeOther)
	}
	if _, err := db.GetSessionByToken(database, token); err == nil {
		t.Fatal("session still exists after logout")
	}
}
