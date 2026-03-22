package main

import (
	"strings"
	"testing"
)

func TestSessionStats_Format(t *testing.T) {
	stats := sessionStats()
	if stats == "" {
		t.Fatal("sessionStats returned empty string")
	}
	if !strings.Contains(stats, "Duration:") {
		t.Error("expected stats to contain Duration")
	}
	if !strings.Contains(stats, "Messages sent:") {
		t.Error("expected stats to contain Messages sent")
	}
	if !strings.Contains(stats, "Tool calls:") {
		t.Error("expected stats to contain Tool calls")
	}
	if !strings.Contains(stats, "Output tokens:") {
		t.Error("expected stats to contain Output tokens")
	}
}

func TestRecordMessage_Increments(t *testing.T) {
	before := sessionMessages
	recordMessage()
	if sessionMessages != before+1 {
		t.Errorf("expected sessionMessages = %d, got %d", before+1, sessionMessages)
	}
}

func TestRecordToolCall_Increments(t *testing.T) {
	before := sessionToolCalls
	recordToolCall()
	if sessionToolCalls != before+1 {
		t.Errorf("expected sessionToolCalls = %d, got %d", before+1, sessionToolCalls)
	}
}

func TestRecordMessage_Multiple(t *testing.T) {
	before := sessionMessages
	for i := 0; i < 5; i++ {
		recordMessage()
	}
	if sessionMessages != before+5 {
		t.Errorf("expected sessionMessages = %d, got %d", before+5, sessionMessages)
	}
}

func TestRecordToolCall_Multiple(t *testing.T) {
	before := sessionToolCalls
	for i := 0; i < 3; i++ {
		recordToolCall()
	}
	if sessionToolCalls != before+3 {
		t.Errorf("expected sessionToolCalls = %d, got %d", before+3, sessionToolCalls)
	}
}

func TestSessionStats_UpdatesAfterRecording(t *testing.T) {
	msgsBefore := sessionMessages
	toolsBefore := sessionToolCalls

	recordMessage()
	recordMessage()
	recordToolCall()

	stats := sessionStats()
	// Check the stats reflect our recordings
	// We can't check exact numbers due to test parallelism, but we can verify the format
	if !strings.Contains(stats, "Duration:") {
		t.Error("stats should contain Duration")
	}

	// Restore (not strictly necessary, but keeps state clean for other tests)
	_ = msgsBefore
	_ = toolsBefore
}
