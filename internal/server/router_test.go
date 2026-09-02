package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"cotests/internal/db"
	"cotests/internal/testutil"
)

func TestLoginSetsSessionAndHTMXRedirect(t *testing.T) {
	router, database := newTestRouter(t)
	testutil.CreateUser(t, database, "user@example.com")
	csrf := getCSRFCookie(t, router, "/login")
	form := url.Values{"email": {"user@example.com"}, "password": {"password1"}, "csrf_token": {csrf.Value}}
	req := formRequest(http.MethodPost, "/login", form, csrf)
	req.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusOK || response.Header().Get("HX-Redirect") != "/" {
		t.Fatalf("login response = %d redirect %q", response.Code, response.Header().Get("HX-Redirect"))
	}
	if cookie := sessionCookie(t, response.Result()); !cookie.HttpOnly {
		t.Fatal("login did not set an HTTP-only session cookie")
	}
}

func TestLoginRejectsInvalidPassword(t *testing.T) {
	router, database := newTestRouter(t)
	testutil.CreateUser(t, database, "user@example.com")
	csrf := getCSRFCookie(t, router, "/login")
	form := url.Values{"email": {"user@example.com"}, "password": {"wrongpass"}, "csrf_token": {csrf.Value}}
	req := formRequest(http.MethodPost, "/login", form, csrf)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHealth(t *testing.T) {
	router, _ := newTestRouter(t)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("health response = %d %q", response.Code, response.Body.String())
	}
}

func TestRegisterSetsSecureSessionAndAdminRole(t *testing.T) {
	router, database := newTestRouter(t)
	csrf := getCSRFCookie(t, router, "/register")
	form := url.Values{"email": {"admin@example.com"}, "password": {"password1"}, "name": {"Admin"}, "csrf_token": {csrf.Value}}
	req := formRequest(http.MethodPost, "/register", form, csrf)
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
	cookie := sessionCookie(t, response.Result())
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unsafe session cookie: %#v", cookie)
	}
}

func TestRegisterRejectsMissingCSRFToken(t *testing.T) {
	router, _ := newTestRouter(t)
	form := url.Values{"email": {"admin@example.com"}, "password": {"password1"}}
	req := formRequest(http.MethodPost, "/register", form)
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
		req := formRequest(http.MethodPost, "/register", form, csrf)
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
	admin := testutil.CreateUser(t, database, "admin@example.com")
	member := testutil.CreateUser(t, database, "member@example.com")
	adminToken := testutil.CreateSession(t, database, admin.ID)
	memberToken := testutil.CreateSession(t, database, member.ID)

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
	user := testutil.CreateUser(t, database, "admin@example.com")
	token := testutil.CreateSession(t, database, user.ID)

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
	logoutRequest := formRequest(http.MethodPost, "/logout", form, &http.Cookie{Name: "session_token", Value: token}, csrf)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d, want %d", logoutResponse.Code, http.StatusSeeOther)
	}
	if _, err := db.GetSessionByToken(database, token); err == nil {
		t.Fatal("session still exists after logout")
	}
}

func TestLogoutRejectsMissingCSRFToken(t *testing.T) {
	router, database := newTestRouter(t)
	user := testutil.CreateUser(t, database, "user@example.com")
	token := testutil.CreateSession(t, database, user.ID)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, formRequest(http.MethodPost, "/logout", url.Values{}, &http.Cookie{Name: "session_token", Value: token}))

	if response.Code != http.StatusForbidden {
		t.Fatalf("logout status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if _, err := db.GetSessionByToken(database, token); err != nil {
		t.Fatalf("session was deleted after invalid logout: %v", err)
	}
}

func TestLogoutAcceptsCSRFHeader(t *testing.T) {
	router, database := newTestRouter(t)
	user := testutil.CreateUser(t, database, "user@example.com")
	token := testutil.CreateSession(t, database, user.ID)
	csrf := getCSRFCookie(t, router, "/", &http.Cookie{Name: "session_token", Value: token})
	req := formRequest(http.MethodPost, "/logout", url.Values{}, &http.Cookie{Name: "session_token", Value: token}, csrf)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	if _, err := db.GetSessionByToken(database, token); err == nil {
		t.Fatal("session still exists after logout")
	}
}

func TestExpiredSessionCannotAccessAdmin(t *testing.T) {
	router, database := newTestRouter(t)
	user := testutil.CreateUser(t, database, "user@example.com")
	token := testutil.CreateSession(t, database, user.ID)
	if err := database.Model(&db.Session{}).Where("user_id = ?", user.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire session: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("expired-session response = %d %q", response.Code, response.Header().Get("Location"))
	}
	if _, err := db.GetSessionByToken(database, token); err == nil {
		t.Fatal("expired session was not deleted")
	}
}

func TestHTMXGuestIsDeniedWithFragment(t *testing.T) {
	router, _ := newTestRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "Please log in") {
		t.Fatalf("HTMX guest response = %d %q", response.Code, response.Body.String())
	}
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{"valid", "user@example.com", "password1", false},
		{"invalid email", "user", "password1", true},
		{"short password", "user@example.com", "short", true},
		{"long password", "user@example.com", strings.Repeat("a", 73), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredentials(tt.email, tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCredentials() error = %v, want error %t", err, tt.wantErr)
			}
		})
	}
}

func TestConstantTimeEqual(t *testing.T) {
	if !constantTimeEqual("same", "same") {
		t.Fatal("equal strings were not equal")
	}
	if constantTimeEqual("same", "different") || constantTimeEqual("same", "same!") {
		t.Fatal("different strings were equal")
	}
}
