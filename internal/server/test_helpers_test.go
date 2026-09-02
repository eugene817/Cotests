package server

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"cotests/internal/testutil"

	"gorm.io/gorm"
)

func newTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()

	database := testutil.NewDatabase(t)
	tpl := template.Must(template.New("pages").Parse(`
{{define "layout"}}{{.Content}}{{end}}
{{define "home"}}home{{end}}
{{define "auth_form"}}{{.Error}}<form><input value="{{.CSRFToken}}"></form>{{end}}
{{define "admin"}}admin{{end}}`))
	return NewRouter(database, http.NotFoundHandler(), tpl, Config{SecureCookies: true}), database
}

func getCSRFCookie(t *testing.T, router http.Handler, path string, cookies ...*http.Cookie) *http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
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

func formRequest(method, path string, values url.Values, cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req
}

func sessionCookie(t *testing.T, response *http.Response) *http.Cookie {
	t.Helper()

	for _, cookie := range response.Cookies() {
		if cookie.Name == "session_token" {
			return cookie
		}
	}
	t.Fatal("session cookie not set")
	return nil
}
