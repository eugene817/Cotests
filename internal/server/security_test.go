package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRequestProtectionRejectsCrossOriginPost(t *testing.T) {
	router, _ := newTestRouter(t)
	csrf := getCSRFCookie(t, router, "/login")
	req := formRequest(http.MethodPost, "/login", url.Values{"email": {"a@example.com"}, "password": {"password1"}, "csrf_token": {csrf.Value}}, csrf)
	req.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.Code)
	}
}
