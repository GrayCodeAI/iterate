package util

import "testing"

func BenchmarkTruncateShort(b *testing.B) {
	s := "hello world"
	for i := 0; i < b.N; i++ {
		Truncate(s, 100)
	}
}

func BenchmarkTruncateLong(b *testing.B) {
	s := string(make([]byte, 10000))
	for i := 0; i < b.N; i++ {
		Truncate(s, 50)
	}
}
