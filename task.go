package main

import (
	"errors"
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
