package server

import (
	"cotests/internal/content"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

// RequestProtection rejects cross-origin or forged browser mutations before a
// handler can alter state. An absent Origin is allowed for ordinary same-site
// form posts; browsers send Origin for cross-origin requests.
func RequestProtection(publicURL *url.URL, h *Handler, config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			// Parse the token under a common cap rather than allowing each handler
			// to consume an unbounded form body first.
			limit := int64(1 << 20)
			if strings.HasPrefix(r.URL.Path, "/api/v1/revisions/") && strings.HasSuffix(r.URL.Path, "/pdf") {
				limit = config.MaxArtifactBytes + (64 << 10)
			}
			if strings.HasPrefix(r.URL.Path, "/api/v1/revisions/") && strings.HasSuffix(r.URL.Path, "/imports") {
				limit = config.MaxImportBytes + (64 << 10)
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			cookie, err := r.Cookie(csrfCookieName)
			if !sameOrigin(r, publicURL) || err != nil || !h.validSignedCSRF(r, cookie.Value) || !validRequestCSRF(r) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					apiError(w, content.ErrForbidden)
					return
				}
				renderAccessError(w, http.StatusForbidden, "Your form has expired. Please refresh and try again.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sameOrigin(r *http.Request, publicURL *url.URL) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	wantHost := r.Host
	wantScheme := "http"
	if r.TLS != nil {
		wantScheme = "https"
	}
	if publicURL != nil {
		wantHost = publicURL.Host
		wantScheme = publicURL.Scheme
	}
	return parsed.Scheme == wantScheme && strings.EqualFold(parsed.Host, wantHost)
}

func (h *Handler) signCSRF(r *http.Request, nonce string) string {
	mac := hmac.New(sha256.New, h.CSRFKey)
	mac.Write([]byte(nonce))
	mac.Write([]byte{0})
	if cookie, err := r.Cookie("session_token"); err == nil {
		mac.Write([]byte(cookie.Value))
	}
	return nonce + "." + hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) validSignedCSRF(r *http.Request, token string) bool {
	nonce, _, ok := strings.Cut(token, ".")
	return ok && len(nonce) == 64 && constantTimeEqual(h.signCSRF(r, nonce), token)
}

func validRequestCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		if err := r.ParseForm(); err != nil {
			return false
		}
		token = r.Form.Get("csrf_token")
	}
	return constantTimeEqual(cookie.Value, token)
}
