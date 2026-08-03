package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
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

// newRouter wires the endpoints. Plain ServeMux keeps the module dependency-free and
// buildable on Go 1.18 (method-aware patterns would need 1.22+).
func newRouter(s *Store) *http.ServeMux {
	mux := http.NewServeMux()
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
	return mux
}

// decodeTask reads and validates the request body, writing the 400 itself when invalid.
func decodeTask(w http.ResponseWriter, r *http.Request) (Task, bool) {
	var t Task
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return Task{}, false
	}
	if err := t.validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return Task{}, false
	}
	return t, true
}

func writeJSON(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
