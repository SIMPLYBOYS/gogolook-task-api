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
}
