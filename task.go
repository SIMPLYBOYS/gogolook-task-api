package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Task is the resource exposed by the API.
type Task struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"` // 0 = incomplete, 1 = completed
}

func (t Task) validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("name is required")
	}
	if t.Status != 0 && t.Status != 1 {
		return errors.New("status must be 0 or 1")
	}
	return nil
}

// Store is the in-memory task storage.
// ponytail: one mutex over one map. Swap for a real DB (or shard the lock) only if
// write throughput ever becomes the bottleneck.
type Store struct {
	mu     sync.RWMutex
	tasks  map[int]Task
	nextID int
}

func NewStore() *Store {
	return &Store{tasks: make(map[int]Task), nextID: 1}
}

func (s *Store) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Create(t Task) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	t.ID = s.nextID
	s.nextID++
	s.tasks[t.ID] = t
	return t
}

func (s *Store) Get(id int) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	return t, ok
}

func (s *Store) Update(id int, t Task) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return Task{}, false
	}
	t.ID = id
	s.tasks[id] = t
	return t, true
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return false
	}
	delete(s.tasks, id)
	return true
}

// newRouter wires the endpoints. Plain ServeMux keeps the module dependency-free and
// buildable on Go 1.18 (method-aware patterns would need 1.22+).
func newRouter(s *Store) *http.ServeMux {
	mux := http.NewServeMux()
	// Least specific pattern, so it only catches paths no other handler claims. Keeps
	// unknown routes in the same JSON error shape as the rest of the API instead of the
	// mux's plain-text "404 page not found".
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "not found")
	})
	// Liveness only: the process is up and serving. Storage is in-process, so there is
	// no dependency whose health could differ from the server's own.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, s.List())
		case http.MethodPost:
			t, ok := decodeTask(w, r)
			if !ok {
				return
			}
			writeJSON(w, http.StatusCreated, s.Create(t))
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/tasks/"))
		if err != nil || id < 1 {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			t, found := s.Get(id)
			if !found {
				writeErr(w, http.StatusNotFound, "task not found")
				return
			}
			writeJSON(w, http.StatusOK, t)
		case http.MethodPut:
			t, ok := decodeTask(w, r)
			if !ok {
				return
			}
			updated, found := s.Update(id, t)
			if !found {
				writeErr(w, http.StatusNotFound, "task not found")
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			if !s.Delete(id) {
				writeErr(w, http.StatusNotFound, "task not found")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	return mux
}

// taskInput mirrors Task with pointers so an absent key is distinguishable from a
// zero value: status 0 is a meaningful state, so "omitted" cannot be inferred from
// the decoded value alone. The spec says a task carries both fields, so a request
// missing one is rejected rather than silently defaulted to incomplete.
type taskInput struct {
	ID     *int    `json:"id"` // accepted so a task read back from GET can be PUT unchanged; ignored
	Name   *string `json:"name"`
	Status *int    `json:"status"`
}

// decodeTask reads and validates the request body, writing the 400 itself when invalid.
func decodeTask(w http.ResponseWriter, r *http.Request) (Task, bool) {
	var in taskInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return Task{}, false
	}
	if in.Name == nil {
		writeErr(w, http.StatusBadRequest, "name is required")
		return Task{}, false
	}
	if in.Status == nil {
		writeErr(w, http.StatusBadRequest, "status is required")
		return Task{}, false
	}

	t := Task{Name: *in.Name, Status: *in.Status}
	if err := t.validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return Task{}, false
	}
	return t, true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
