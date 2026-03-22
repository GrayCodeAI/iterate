package main

import "testing"

func BenchmarkFuzzyHistorySearch(b *testing.B) {
	// Set up test history
	inputHistory = make([]string, 100)
	for i := range inputHistory {
		inputHistory[i] = "test command " + string(rune('a'+i%26))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fuzzyHistorySearch()
	}
}
