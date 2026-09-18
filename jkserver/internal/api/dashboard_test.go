package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
)

// newRouter builds a chi router with auth + dashboard CRUD endpoints.
func newRouter(t *testing.T, d *db.DB) (*chi.Mux, *httptest.Server) {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/api/dashboard/auth/login", LoginHandler(d))
	r.Post("/api/dashboard/auth/logout", LogoutHandler())
	r.Mount("/api/dashboard", DashboardRouter(d, nil, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return r, srv
}

// loginAndGetCookie logs in and returns the session cookie.
func loginAndGetCookie(t *testing.T, srv *httptest.Server, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest("POST", srv.URL+"/api/dashboard/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d, body: %s", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()
	for _, c := range cookie {
		if c.Name == "jkr_session" {
			return c
		}
	}
	t.Fatal("no session cookie after login")
	return nil
}

func TestListProvidersEmpty(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	cookie := loginAndGetCookie(t, srv, "testpwd")

	req := httptest.NewRequest("GET", srv.URL+"/api/dashboard/providers", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list providers: %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	providers, ok := resp["providers"].([]interface{})
	if !ok {
		t.Fatalf("expected providers array")
	}
	if len(providers) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(providers))
	}
}

func TestCreateAndGetProvider(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	cookie := loginAndGetCookie(t, srv, "testpwd")

	// Create provider
	createBody, _ := json.Marshal(map[string]string{"name": "test-provider", "label": "Test Provider"})
	createReq := httptest.NewRequest("POST", srv.URL+"/api/dashboard/providers", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create provider: %d, body: %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]interface{}
	json.NewDecoder(createRec.Body).Decode(&created)
	if created["name"] != "test-provider" {
		t.Fatalf("expected name 'test-provider', got %v", created["name"])
	}

	// Get provider
	getReq := httptest.NewRequest("GET", srv.URL+"/api/dashboard/providers/test-provider", nil)
	getReq.AddCookie(cookie)
	getRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get provider: %d", getRec.Code)
	}
	var got map[string]interface{}
	json.NewDecoder(getRec.Body).Decode(&got)
	if got["id"] != "test-provider" {
		t.Fatalf("expected id 'test-provider', got %v", got["id"])
	}
}

func TestUpdateProvider(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	cookie := loginAndGetCookie(t, srv, "testpwd")

	// Create then update
	createBody, _ := json.Marshal(map[string]string{"name": "upd-prov", "label": "Original"})
	createReq := httptest.NewRequest("POST", srv.URL+"/api/dashboard/providers", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(createRec, createReq)

	updateBody, _ := json.Marshal(map[string]string{"label": "Updated"})
	updateReq := httptest.NewRequest("PUT", srv.URL+"/api/dashboard/providers/upd-prov", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(cookie)
	updateRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update provider: %d", updateRec.Code)
	}
}

func TestDeleteProvider(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	cookie := loginAndGetCookie(t, srv, "testpwd")

	// Create then delete
	createBody, _ := json.Marshal(map[string]string{"name": "del-prov"})
	createReq := httptest.NewRequest("POST", srv.URL+"/api/dashboard/providers", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(createRec, createReq)

	deleteReq := httptest.NewRequest("DELETE", srv.URL+"/api/dashboard/providers/del-prov", nil)
	deleteReq.AddCookie(cookie)
	deleteRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete provider: %d", deleteRec.Code)
	}

	// Verify gone
	getReq := httptest.NewRequest("GET", srv.URL+"/api/dashboard/providers/del-prov", nil)
	getReq.AddCookie(cookie)
	getRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getRec.Code)
	}
}

func TestSettingsGetSet(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	cookie := loginAndGetCookie(t, srv, "testpwd")

	// Get settings (should return empty/default)
	getReq := httptest.NewRequest("GET", srv.URL+"/api/dashboard/settings", nil)
	getReq.AddCookie(cookie)
	getRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get settings: %d", getRec.Code)
	}

	// Set settings
	setBody, _ := json.Marshal(map[string]interface{}{"dark_mode": true, "theme": "ocean"})
	setReq := httptest.NewRequest("PUT", srv.URL+"/api/dashboard/settings", bytes.NewReader(setBody))
	setReq.Header.Set("Content-Type", "application/json")
	setReq.AddCookie(cookie)
	setRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("set settings: %d", setRec.Code)
	}

	// Get again - should reflect changes
	getReq2 := httptest.NewRequest("GET", srv.URL+"/api/dashboard/settings", nil)
	getReq2.AddCookie(cookie)
	getRec2 := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(getRec2, getReq2)
	if getRec2.Code != http.StatusOK {
		t.Fatalf("get settings after set: %d", getRec2.Code)
	}
}

func TestAuthBlocksUnauthenticated(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	req := httptest.NewRequest("GET", srv.URL+"/api/dashboard/providers", nil)
	rec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestConnectionsCRUD(t *testing.T) {
	d := newTestDB(t, hashPassword(t, "testpwd"))
	_, srv := newRouter(t, d)

	cookie := loginAndGetCookie(t, srv, "testpwd")

	// List connections (empty)
	listReq := httptest.NewRequest("GET", srv.URL+"/api/dashboard/connections", nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list connections: %d", listRec.Code)
	}

	// Create connection
	createBody, _ := json.Marshal(map[string]string{
		"provider_id": "openai",
		"label":       "test-conn",
		"auth_type":   "api_key",
	})
	createReq := httptest.NewRequest("POST", srv.URL+"/api/dashboard/connections", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create connection: %d, body: %s", createRec.Code, createRec.Body.String())
	}

	// List again - should have 1
	listReq2 := httptest.NewRequest("GET", srv.URL+"/api/dashboard/connections", nil)
	listReq2.AddCookie(cookie)
	listRec2 := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(listRec2, listReq2)
	var resp map[string]interface{}
	json.NewDecoder(listRec2.Body).Decode(&resp)
	conns, ok := resp["connections"].([]interface{})
	if !ok {
		t.Fatalf("expected connections array")
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
}
