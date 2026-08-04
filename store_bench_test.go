package main

import (
	"fmt"
	"testing"
)

var sink []Task

func seed(n int) *Store {
	s := NewStore()
	for i := 0; i < n; i++ {
		s.Create(Task{Name: "task", Status: 0})
	}
	return s
}

// List sorts on every call, so its cost grows with the number of stored tasks.
// This is the ceiling behind the "swap for a real DB" note on Store.
func BenchmarkStoreList(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("tasks=%d", n), func(b *testing.B) {
			s := seed(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sink = s.List()
			}
		})
	}
}

// Reads share the RWMutex, so throughput should scale with GOMAXPROCS.
func BenchmarkStoreListParallel(b *testing.B) {
	s := seed(1000)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sink = s.List()
		}
	})
}

// Writes take the exclusive lock, so they serialize no matter how many cores
// are available. Update keeps the store at a fixed size, so the measurement is
// lock contention rather than a map that grows during the run.
func BenchmarkStoreUpdateParallel(b *testing.B) {
	s := seed(1000)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 1
		for pb.Next() {
			s.Update(id, Task{Name: "task", Status: 1})
			if id++; id > 1000 {
				id = 1
			}
		}
	})
}
