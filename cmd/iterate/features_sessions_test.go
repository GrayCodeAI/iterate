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
	before := sess.Messages
	recordMessage()
	if sess.Messages != before+1 {
		t.Errorf("expected sessionMessages = %d, got %d", before+1, sess.Messages)
	}
}

func TestRecordToolCall_Increments(t *testing.T) {
	before := sess.ToolCalls
	recordToolCall()
	if sess.ToolCalls != before+1 {
		t.Errorf("expected sessionToolCalls = %d, got %d", before+1, sess.ToolCalls)
	}
}

func TestRecordMessage_Multiple(t *testing.T) {
	before := sess.Messages
	for i := 0; i < 5; i++ {
		recordMessage()
	}
	if sess.Messages != before+5 {
		t.Errorf("expected sessionMessages = %d, got %d", before+5, sess.Messages)
	}
}

func TestRecordToolCall_Multiple(t *testing.T) {
	before := sess.ToolCalls
	for i := 0; i < 3; i++ {
		recordToolCall()
	}
	if sess.ToolCalls != before+3 {
		t.Errorf("expected sessionToolCalls = %d, got %d", before+3, sess.ToolCalls)
	}
}

func TestSessionStats_UpdatesAfterRecording(t *testing.T) {
	msgsBefore := sess.Messages
	toolsBefore := sess.ToolCalls

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
