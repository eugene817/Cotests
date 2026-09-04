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

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "Invalid email or password") {
		t.Fatalf("login response = %d %q", response.Code, response.Body.String())
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

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin/contests", nil)
	adminRequest.AddCookie(&http.Cookie{Name: "session_token", Value: adminToken})
	adminResponse := httptest.NewRecorder()
	router.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK || !strings.Contains(adminResponse.Body.String(), "admin") {
		t.Fatalf("admin response = %d %q", adminResponse.Code, adminResponse.Body.String())
	}
}

func TestAdminRedirectsToCanonicalContestURL(t *testing.T) {
	router, database := newTestRouter(t)
	admin := testutil.CreateUser(t, database, "admin@example.com")
	token := testutil.CreateSession(t, database, admin.ID)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != "/admin/contests" {
		t.Fatalf("admin redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestAdminSeriesRouteRequiresMatchingContest(t *testing.T) {
	router, database := newTestRouter(t)
	admin := testutil.CreateUser(t, database, "admin@example.com")
	token := testutil.CreateSession(t, database, admin.ID)
	first := &db.Contest{Title: "First"}
	second := &db.Contest{Title: "Second"}
	if err := db.CreateContest(database, first); err != nil {
		t.Fatalf("create first contest: %v", err)
	}
	if err := db.CreateContest(database, second); err != nil {
		t.Fatalf("create second contest: %v", err)
	}
	if err := db.CreateSeries(database, &db.Series{ContestID: first.ID, Title: "Series", Position: 0}); err != nil {
		t.Fatalf("create series: %v", err)
	}
	csrf := getCSRFCookie(t, router, "/admin/contests", &http.Cookie{Name: "session_token", Value: token})
	request := formRequest(http.MethodPost, "/admin/contests/2/series/1", url.Values{
		"title": {"Changed"}, "position": {"0"}, "csrf_token": {csrf.Value},
	}, &http.Cookie{Name: "session_token", Value: token}, csrf)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("mismatched series route = %d", response.Code)
	}
}

func TestPublicContestRoutesHideDrafts(t *testing.T) {
	router, database := newTestRouter(t)
	published := &db.Contest{Title: "Published", Visibility: db.ContestPublished}
	draft := &db.Contest{Title: "Draft", Visibility: db.ContestDraft}
	if err := db.CreateContest(database, published); err != nil {
		t.Fatalf("create published contest: %v", err)
	}
	if err := db.CreateContest(database, draft); err != nil {
		t.Fatalf("create draft contest: %v", err)
	}

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/contests", nil))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "public contests") {
		t.Fatalf("public list = %d %q", listResponse.Code, listResponse.Body.String())
	}

	publishedResponse := httptest.NewRecorder()
	router.ServeHTTP(publishedResponse, httptest.NewRequest(http.MethodGet, "/contests/1", nil))
	if publishedResponse.Code != http.StatusOK {
		t.Fatalf("published contest = %d", publishedResponse.Code)
	}
	draftResponse := httptest.NewRecorder()
	router.ServeHTTP(draftResponse, httptest.NewRequest(http.MethodGet, "/contests/2", nil))
	if draftResponse.Code != http.StatusNotFound {
		t.Fatalf("draft contest = %d", draftResponse.Code)
	}
}

func TestAdminCanManageContestAndSeries(t *testing.T) {
	router, database := newTestRouter(t)
	admin := testutil.CreateUser(t, database, "admin@example.com")
	token := testutil.CreateSession(t, database, admin.ID)
	session := &http.Cookie{Name: "session_token", Value: token}
	csrf := getCSRFCookie(t, router, "/admin/contests", session)

	createContest := formRequest(http.MethodPost, "/admin/contests", url.Values{
		"title": {"Spring Contest"}, "description": {"Practice"}, "visibility": {"draft"}, "csrf_token": {csrf.Value},
	}, session, csrf)
	contestResponse := httptest.NewRecorder()
	router.ServeHTTP(contestResponse, createContest)
	if contestResponse.Code != http.StatusSeeOther || contestResponse.Header().Get("Location") != "/admin/contests/1" {
		t.Fatalf("create contest = %d %q", contestResponse.Code, contestResponse.Header().Get("Location"))
	}

	createSeries := formRequest(http.MethodPost, "/admin/contests/1/series", url.Values{
		"title": {"Warm-up"}, "position": {"0"}, "csrf_token": {csrf.Value},
	}, session, csrf)
	seriesResponse := httptest.NewRecorder()
	router.ServeHTTP(seriesResponse, createSeries)
	if seriesResponse.Code != http.StatusSeeOther {
		t.Fatalf("create series = %d", seriesResponse.Code)
	}

	updateContest := formRequest(http.MethodPost, "/admin/contests/1", url.Values{
		"title": {"Updated Contest"}, "description": {"Practice"}, "visibility": {"published"}, "csrf_token": {csrf.Value},
	}, session, csrf)
	updateContestResponse := httptest.NewRecorder()
	router.ServeHTTP(updateContestResponse, updateContest)
	if updateContestResponse.Code != http.StatusSeeOther {
		t.Fatalf("update contest = %d", updateContestResponse.Code)
	}

	updateSeries := formRequest(http.MethodPost, "/admin/contests/1/series/1", url.Values{
		"title": {"Updated warm-up"}, "position": {"1"}, "csrf_token": {csrf.Value},
	}, session, csrf)
	updateSeriesResponse := httptest.NewRecorder()
	router.ServeHTTP(updateSeriesResponse, updateSeries)
	if updateSeriesResponse.Code != http.StatusSeeOther {
		t.Fatalf("update series = %d", updateSeriesResponse.Code)
	}

	deleteSeries := formRequest(http.MethodPost, "/admin/contests/1/series/1/delete", url.Values{"csrf_token": {csrf.Value}}, session, csrf)
	deleteSeriesResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteSeriesResponse, deleteSeries)
	if deleteSeriesResponse.Code != http.StatusSeeOther {
		t.Fatalf("delete series = %d", deleteSeriesResponse.Code)
	}
	var count int64
	if err := database.Model(&db.Series{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("series after delete = %d, err = %v", count, err)
	}

	deleteContest := formRequest(http.MethodPost, "/admin/contests/1/delete", url.Values{"csrf_token": {csrf.Value}}, session, csrf)
	deleteContestResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteContestResponse, deleteContest)
	if deleteContestResponse.Code != http.StatusSeeOther || deleteContestResponse.Header().Get("Location") != "/admin/contests" {
		t.Fatalf("delete contest = %d %q", deleteContestResponse.Code, deleteContestResponse.Header().Get("Location"))
	}
}

func TestAdminContestRoutesRequireCSRFAndAdminRole(t *testing.T) {
	router, database := newTestRouter(t)
	admin := testutil.CreateUser(t, database, "admin@example.com")
	member := testutil.CreateUser(t, database, "member@example.com")
	adminToken := testutil.CreateSession(t, database, admin.ID)
	memberToken := testutil.CreateSession(t, database, member.ID)
	values := url.Values{"title": {"Contest"}, "visibility": {"draft"}}

	missingCSRF := httptest.NewRecorder()
	router.ServeHTTP(missingCSRF, formRequest(http.MethodPost, "/admin/contests", values, &http.Cookie{Name: "session_token", Value: adminToken}))
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", missingCSRF.Code)
	}

	memberResponse := httptest.NewRecorder()
	router.ServeHTTP(memberResponse, formRequest(http.MethodPost, "/admin/contests", values, &http.Cookie{Name: "session_token", Value: memberToken}))
	if memberResponse.Code != http.StatusForbidden {
		t.Fatalf("member status = %d", memberResponse.Code)
	}
}

func TestAdminHTMXCreateAndDeleteSeriesReturnFragments(t *testing.T) {
	router, database := newTestRouter(t)
	admin := testutil.CreateUser(t, database, "admin@example.com")
	token := testutil.CreateSession(t, database, admin.ID)
	session := &http.Cookie{Name: "session_token", Value: token}
	csrf := getCSRFCookie(t, router, "/admin/contests", session)

	createContest := formRequest(http.MethodPost, "/admin/contests", url.Values{
		"title": {"HTMX Contest"}, "visibility": {"draft"}, "csrf_token": {csrf.Value},
	}, session, csrf)
	createContest.Header.Set("HX-Request", "true")
	contestResponse := httptest.NewRecorder()
	router.ServeHTTP(contestResponse, createContest)
	if contestResponse.Code != http.StatusOK || contestResponse.Header().Get("HX-Retarget") != "#contest-list" || contestResponse.Header().Get("HX-Reswap") != "afterbegin" || !strings.Contains(contestResponse.Body.String(), "HTMX Contest") {
		t.Fatalf("HTMX create contest = %d %q %q", contestResponse.Code, contestResponse.Header().Get("HX-Retarget"), contestResponse.Body.String())
	}

	series := &db.Series{ContestID: 1, Title: "Series", Position: 0}
	if err := db.CreateSeries(database, series); err != nil {
		t.Fatalf("create series: %v", err)
	}
	deleteSeries := formRequest(http.MethodPost, "/admin/contests/1/series/1/delete", url.Values{"csrf_token": {csrf.Value}}, session, csrf)
	deleteSeries.Header.Set("HX-Request", "true")
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteSeries)
	if deleteResponse.Code != http.StatusOK || deleteResponse.Body.Len() != 0 {
		t.Fatalf("HTMX delete series = %d %q", deleteResponse.Code, deleteResponse.Body.String())
	}

	invalidContest := formRequest(http.MethodPost, "/admin/contests", url.Values{
		"title": {""}, "visibility": {"draft"}, "csrf_token": {csrf.Value},
	}, session, csrf)
	invalidContest.Header.Set("HX-Request", "true")
	invalidResponse := httptest.NewRecorder()
	router.ServeHTTP(invalidResponse, invalidContest)
	if invalidResponse.Code != http.StatusBadRequest || !strings.Contains(invalidResponse.Body.String(), "contest title is required") {
		t.Fatalf("HTMX validation error = %d %q", invalidResponse.Code, invalidResponse.Body.String())
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

func TestHTMXLogoutRedirectsHome(t *testing.T) {
	router, database := newTestRouter(t)
	user := testutil.CreateUser(t, database, "user@example.com")
	token := testutil.CreateSession(t, database, user.ID)
	csrf := getCSRFCookie(t, router, "/", &http.Cookie{Name: "session_token", Value: token})
	req := formRequest(http.MethodPost, "/logout", url.Values{"csrf_token": {csrf.Value}}, &http.Cookie{Name: "session_token", Value: token}, csrf)
	req.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusOK || response.Header().Get("HX-Redirect") != "/" {
		t.Fatalf("HTMX logout = %d %q", response.Code, response.Header().Get("HX-Redirect"))
	}
	if _, err := db.GetSessionByToken(database, token); err == nil {
		t.Fatal("session still exists after HTMX logout")
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
