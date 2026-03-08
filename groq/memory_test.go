package groq

import (
	"reflect"
	"testing"
)

func TestMemoryCache(t *testing.T) {
	chatID := int64(9999)

	// Ensure clear starts us fresh
	ClearHistory(chatID)

	h := getHistory(chatID)
	if h == nil {
		t.Fatal("Expected history to be initialized, got nil")
	}

	if len(h.GetMessages()) != 0 {
		t.Fatalf("Expected 0 messages, got %d", len(h.GetMessages()))
	}

	h.AddMessage("user", "Hello")
	h.AddMessage("assistant", "Hi there")

	msgs := h.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Errorf("Unexpected message 0: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Hi there" {
		t.Errorf("Unexpected message 1: %+v", msgs[1])
	}

	// Test max length eviction (maxLen is 4)
	h.AddMessage("user", "Q1")
	h.AddMessage("assistant", "A1")
	h.AddMessage("user", "Q2")
	h.AddMessage("assistant", "A2")

	msgs = h.GetMessages()
	if len(msgs) != 4 {
		t.Fatalf("Expected strictly 4 messages due to maxLen, got %d", len(msgs))
	}

	// The first two "Hello" and "Hi there" should be evicted
	expected := []Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "A2"},
	}

	if !reflect.DeepEqual(msgs, expected) {
		t.Errorf("Expected evicted history %+v, got %+v", expected, msgs)
	}

	// Test ClearHistory
	ClearHistory(chatID)

	h2 := getHistory(chatID)
	if len(h2.GetMessages()) != 0 {
		t.Fatalf("Expected history to be empty after clear, got %d", len(h2.GetMessages()))
	}
}
