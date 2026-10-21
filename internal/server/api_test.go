package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cotests/internal/db"
	"cotests/internal/testutil"
)

type apiClient struct {
	t             *testing.T
	router        http.Handler
	session, csrf *http.Cookie
}

func newAPIClient(t *testing.T, router http.Handler, session string) *apiClient {
	c := &apiClient{t: t, router: router, session: &http.Cookie{Name: "session_token", Value: session}}
	c.csrf = getCSRFCookie(t, router, "/api/v1/csrf", c.session)
	return c
}
func (c *apiClient) request(method, path, body string, want int) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrf.Value)
	req.AddCookie(c.csrf)
	req.AddCookie(c.session)
	response := httptest.NewRecorder()
	c.router.ServeHTTP(response, req)
	if response.Code != want {
		c.t.Fatalf("%s %s: %d want %d: %s", method, path, response.Code, want, response.Body)
	}
	return response
}
func (c *apiClient) upload(path string, data []byte, want int) *httptest.ResponseRecorder {
	c.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "upload")
	if err != nil {
		c.t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = writer.Close()
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", c.csrf.Value)
	req.AddCookie(c.csrf)
	req.AddCookie(c.session)
	response := httptest.NewRecorder()
	c.router.ServeHTTP(response, req)
	if response.Code != want {
		c.t.Fatalf("upload %s: %d want %d: %s", path, response.Code, want, response.Body)
	}
	return response
}
func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestContentAPIEndToEnd(t *testing.T) {
	router, database := newTestRouter(t)
	admin := testutil.CreateAdmin(t, database, "admin@api.test")
	author := testutil.CreateUser(t, database, "author@api.test")
	participant := testutil.CreateUser(t, database, "participant@api.test")
	other := testutil.CreateUser(t, database, "other@api.test")
	ac := newAPIClient(t, router, testutil.CreateSession(t, database, admin.ID))
	oc := newAPIClient(t, router, testutil.CreateSession(t, database, author.ID))
	pc := newAPIClient(t, router, testutil.CreateSession(t, database, participant.ID))
	xc := newAPIClient(t, router, testutil.CreateSession(t, database, other.ID))
	contest := decodeResponse[db.Contest](t, ac.request("POST", "/api/v1/contests", `{"title":"Contest","visibility":"published"}`, 200))
	cp := fmt.Sprintf("/api/v1/contests/%d", contest.ID)
	ac.request("PUT", cp+"/members", fmt.Sprintf(`{"user_id":%d,"role":"organizer"}`, author.ID), 204)
	oc.request("PUT", cp+"/members", fmt.Sprintf(`{"user_id":%d,"role":"participant"}`, participant.ID), 204)
	oc.request("PUT", cp+"/members", fmt.Sprintf(`{"user_id":%d,"role":"organizer"}`, other.ID), 403)
	xc.request("PUT", cp, `{"title":"Stolen","visibility":"published"}`, 403)
	series := decodeResponse[db.Series](t, oc.request("POST", cp+"/series", `{"title":"Round","position":0}`, 200))
	sp := fmt.Sprintf("/api/v1/series/%d", series.ID)
	created := decodeResponse[struct {
		Problem  db.Problem
		Revision db.ProblemRevision
	}](t, oc.request("POST", "/api/v1/problems", `{"title":"Sum","code":"sum","statement":"Add integers","languages":["cpp"]}`, 200))
	rp := fmt.Sprintf("/api/v1/revisions/%d", created.Revision.ID)
	pc.request("GET", rp, "", 403)
	xc.request("POST", sp+"/problems", fmt.Sprintf(`{"revision_id":%d,"code":"A","position":0}`, created.Revision.ID), 403)
	oc.request("POST", sp+"/problems", fmt.Sprintf(`{"revision_id":%d,"code":"A","position":0}`, created.Revision.ID), 409)
	sample := decodeResponse[db.TestCase](t, oc.request("POST", rp+"/tests", `{"position":0,"is_sample":true,"input":"1 2","output":"3"}`, 200))
	hidden := decodeResponse[db.TestCase](t, oc.request("POST", rp+"/tests", `{"position":1,"input":"99","output":"100"}`, 200))
	oc.upload(rp+"/pdf", []byte("%PDF-1.7\nstatement"), 200)
	oc.request("POST", rp+"/publish", "", 204)
	oc.request("POST", rp+"/tests", `{"position":2,"input":"x","output":"y"}`, 409)
	oc.request("DELETE", fmt.Sprintf("%s/tests/%d", rp, hidden.ID), "", 409)
	oc.upload(rp+"/pdf", []byte("%PDF-1.7\nchanged"), 409)
	assignment := decodeResponse[db.SeriesProblem](t, oc.request("POST", sp+"/problems", fmt.Sprintf(`{"revision_id":%d,"code":"A","position":0}`, created.Revision.ID), 200))
	ap := fmt.Sprintf("/api/v1/assignments/%d", assignment.ID)
	response := pc.request("GET", ap, "", 200)
	if strings.Contains(response.Body.String(), "checker") || strings.Contains(response.Body.String(), "99") {
		t.Fatalf("private metadata leaked: %s", response.Body)
	}
	xc.request("GET", ap, "", 403)
	pc.request("GET", fmt.Sprintf("/api/v1/artifacts/%d", sample.InputArtifactID), "", 200)
	pc.request("GET", fmt.Sprintf("/api/v1/artifacts/%d", hidden.InputArtifactID), "", 403)
	oc.request("GET", fmt.Sprintf("/api/v1/artifacts/%d", hidden.InputArtifactID), "", 200)
	second := decodeResponse[db.Series](t, ac.request("POST", cp+"/series", `{"title":"Second","position":1,"release_at":"2099-01-01T00:00:00Z"}`, 200))
	oc.request("POST", fmt.Sprintf("/api/v1/series/%d/problems", second.ID), fmt.Sprintf(`{"revision_id":%d,"code":"A","position":0}`, created.Revision.ID), 200)
	oc.request("PUT", cp+"/series/order", `{"ids":[2,1]}`, 204)
	oc.request("PUT", cp+"/series/order", `{"ids":[1,1]}`, 400)
	oc.request("PUT", sp, `{"title":"Round","position":1,"release_at":"2099-01-01T00:00:00Z"}`, 200)
	pc.request("GET", ap, "", 403)
	pc.request("GET", fmt.Sprintf("/api/v1/artifacts/%d", sample.InputArtifactID), "", 403)
	oc.request("GET", ap, "", 200)
	// A separate organizer has no authority over this author's reusable library.
	ac.request("PUT", cp+"/members", fmt.Sprintf(`{"user_id":%d,"role":"organizer"}`, other.ID), 204)
	xc.request("GET", rp, "", 403)
	xc.request("POST", sp+"/problems", fmt.Sprintf(`{"revision_id":%d,"code":"B","position":1}`, created.Revision.ID), 403)
	oc.request("GET", "/api/v1/problems?limit=0", "", 400)
	oc.request("POST", "/api/v1/problems", `{"title":"x","code":"x","extra":true}`, 400)
}

func TestZIPPreviewCommitAPI(t *testing.T) {
	router, database := newTestRouter(t)
	actor := testutil.CreateAdmin(t, database, "zip@api.test")
	client := newAPIClient(t, router, testutil.CreateSession(t, database, actor.ID))
	created := decodeResponse[struct {
		Problem  db.Problem
		Revision db.ProblemRevision
	}](t, client.request("POST", "/api/v1/problems", `{"title":"ZIP","code":"zip","statement":"s"}`, 200))
	rp := fmt.Sprintf("/api/v1/revisions/%d", created.Revision.ID)
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range []string{"01.in", "01.out"} {
		file, _ := writer.Create(name)
		_, _ = file.Write([]byte("test"))
	}
	_ = writer.Close()
	preview := decodeResponse[struct {
		ID uint `json:"id"`
	}](t, client.upload(rp+"/imports", archive.Bytes(), 200))
	client.request("GET", rp+"/tests", "", 200)
	commit := fmt.Sprintf("%s/imports/%d/commit", rp, preview.ID)
	client.request("POST", commit, "", 204)
	client.request("POST", commit, "", 204)
	items := decodeResponse[[]db.TestCase](t, client.request("GET", rp+"/tests", "", 200))
	if len(items) != 1 {
		t.Fatalf("duplicate import: %d", len(items))
	}
	draft := decodeResponse[db.ProblemRevision](t, client.request("POST", fmt.Sprintf("/api/v1/problems/%d/revisions", created.Problem.ID), "", 200))
	client.request("POST", fmt.Sprintf("/api/v1/revisions/%d/imports/%d/commit", draft.ID, preview.ID), "", 404)
}

func TestAPIRequestProtection(t *testing.T) {
	router, database := newTestRouter(t)
	actor := testutil.CreateAdmin(t, database, "security@api.test")
	client := newAPIClient(t, router, testutil.CreateSession(t, database, actor.ID))
	for _, test := range []struct{ name, token, session, origin string }{
		{"unsigned", "forged", client.session.Value, ""},
		{"other session", client.csrf.Value, "different-session", ""},
		{"cross scheme", client.csrf.Value, client.session.Value, "https://example.com"},
		{"cross host", client.csrf.Value, client.session.Value, "http://attacker.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/contests", strings.NewReader(`{"title":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", test.token)
			req.Header.Set("Origin", test.origin)
			req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: test.token})
			req.AddCookie(&http.Cookie{Name: "session_token", Value: test.session})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != 403 || !strings.Contains(response.Body.String(), `"code":"forbidden"`) {
				t.Fatalf("response: %d %s", response.Code, response.Body)
			}
		})
	}
	client.request("POST", "/api/v1/contests", `{"title":"`+strings.Repeat("a", 1<<20)+`"}`, 413)
	for _, path := range []string{"/static/templates/layout.html", "/static/templates/", "/static/css/../templates/layout.html"} {
		client.request("GET", path, "", 404)
	}
	client.request("GET", "/live", "", 200)
	pool, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	client.request("GET", "/live", "", 200)
	client.request("GET", "/health", "", 503)
}
