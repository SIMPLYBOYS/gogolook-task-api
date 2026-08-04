package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// do sends a request through the router and returns the recorder.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) Task {
	t.Helper()
	var got Task
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json response %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestHealthz(t *testing.T) {
	rec := do(t, newRouter(NewStore()), http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("healthz: got %d %q", rec.Code, rec.Body.String())
	}
}

// A 404 from an unrouted path must still be JSON — a bare status check would pass
// even without the catch-all, since ServeMux 404s on its own.
func TestUnknownPathReturnsJSONError(t *testing.T) {
	rec := do(t, newRouter(NewStore()), http.MethodGet, "/nope", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("unknown path: got %d %q", rec.Code, rec.Body.String())
	}
}

func TestCRUDLifecycle(t *testing.T) {
	h := newRouter(NewStore())

	// empty list is [] not null
	rec := do(t, h, http.MethodGet, "/tasks", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("list empty: got %d %q", rec.Code, rec.Body.String())
	}

	// create
	rec = do(t, h, http.MethodPost, "/tasks", `{"name":"buy milk","status":0}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d %q", rec.Code, rec.Body.String())
	}
	created := decode(t, rec)
	if created.ID != 1 || created.Name != "buy milk" || created.Status != 0 {
		t.Fatalf("create: unexpected task %+v", created)
	}

	// ids increment
	rec = do(t, h, http.MethodPost, "/tasks", `{"name":"walk dog","status":1}`)
	if second := decode(t, rec); second.ID != 2 {
		t.Fatalf("create: want id 2, got %d", second.ID)
	}

	// list returns both, ordered by id
	rec = do(t, h, http.MethodGet, "/tasks", "")
	var list []Task
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
		t.Fatalf("list: unexpected %+v", list)
	}

	// fetch one
	rec = do(t, h, http.MethodGet, "/tasks/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get one: got %d %q", rec.Code, rec.Body.String())
	}
	if one := decode(t, rec); one.ID != 1 || one.Name != "buy milk" {
		t.Fatalf("get one: unexpected task %+v", one)
	}

	// update
	rec = do(t, h, http.MethodPut, "/tasks/1", `{"name":"buy oat milk","status":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: got %d %q", rec.Code, rec.Body.String())
	}
	if updated := decode(t, rec); updated.ID != 1 || updated.Name != "buy oat milk" || updated.Status != 1 {
		t.Fatalf("update: unexpected task %+v", updated)
	}

	// delete, then it is gone
	if rec = do(t, h, http.MethodDelete, "/tasks/1", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d %q", rec.Code, rec.Body.String())
	}
	if rec = do(t, h, http.MethodDelete, "/tasks/1", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: want 404, got %d", rec.Code)
	}
	rec = do(t, h, http.MethodGet, "/tasks", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != 2 {
		t.Fatalf("list after delete: unexpected %+v", list)
	}
}

func TestRequestErrors(t *testing.T) {
	h := newRouter(NewStore())
	do(t, h, http.MethodPost, "/tasks", `{"name":"seed","status":0}`) // id 1 exists

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"missing name", http.MethodPost, "/tasks", `{"status":0}`, http.StatusBadRequest},
		{"missing status", http.MethodPost, "/tasks", `{"name":"x"}`, http.StatusBadRequest},
		{"missing status on update", http.MethodPut, "/tasks/1", `{"name":"x"}`, http.StatusBadRequest},
		{"id echoed back is accepted", http.MethodPut, "/tasks/1", `{"id":1,"name":"x","status":1}`, http.StatusOK},
		{"blank name", http.MethodPost, "/tasks", `{"name":"   ","status":0}`, http.StatusBadRequest},
		{"status out of enum", http.MethodPost, "/tasks", `{"name":"x","status":2}`, http.StatusBadRequest},
		{"status wrong type", http.MethodPost, "/tasks", `{"name":"x","status":"done"}`, http.StatusBadRequest},
		{"malformed json", http.MethodPost, "/tasks", `{`, http.StatusBadRequest},
		{"unknown field", http.MethodPost, "/tasks", `{"name":"x","status":0,"oops":1}`, http.StatusBadRequest},
		{"update missing task", http.MethodPut, "/tasks/999", `{"name":"x","status":0}`, http.StatusNotFound},
		{"update invalid body", http.MethodPut, "/tasks/1", `{"name":"","status":0}`, http.StatusBadRequest},
		{"non-numeric id", http.MethodPut, "/tasks/abc", `{"name":"x","status":0}`, http.StatusNotFound},
		{"get missing task", http.MethodGet, "/tasks/999", "", http.StatusNotFound},
		{"delete missing task", http.MethodDelete, "/tasks/999", "", http.StatusNotFound},
		{"method not allowed on collection", http.MethodPatch, "/tasks", "", http.StatusMethodNotAllowed},
		{"method not allowed on item", http.MethodPatch, "/tasks/1", "", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, tc.method, tc.path, tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("want %d, got %d (%q)", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestStoreIsConcurrencySafe(t *testing.T) {
	s := NewStore()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			s.Create(Task{Name: "x", Status: 1})
			s.List()
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	if got := len(s.List()); got != 50 {
		t.Fatalf("want 50 tasks, got %d", got)
	}
}
