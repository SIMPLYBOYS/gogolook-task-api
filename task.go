package main

import (
	"errors"
	"strings"
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
