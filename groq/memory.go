package groq

import (
	"sync"
)

// Message is the standard Groq/OpenAI chat message format.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ConversationHistory represents a rolling window of recent chat messages.
type ConversationHistory struct {
	mu       sync.Mutex
	messages []Message
	maxLen   int
}

var (
	// memCache provides an in-memory storage of conversation histories mapped by chatID.
	// Since this is specifically designed for low concurrency (the user's mother) on Cloud Run,
	// this sync.Map is extremely efficient and requires zero external Redis overhead.
	memCache sync.Map
)

// getHistory fetches or initializes the conversation history for a given chat ID.
// It retains the last 4 user/bot interactions.
func getHistory(chatID int64) *ConversationHistory {
	val, ok := memCache.Load(chatID)
	if !ok {
		h := &ConversationHistory{
			messages: make([]Message, 0, 4),
			maxLen:   4, // Keep the last 4 messages in context (~2 full turns)
		}
		memCache.Store(chatID, h)
		return h
	}
	return val.(*ConversationHistory)
}

// AddMessage appends a message to the chat's history, popping old ones to maintain max length.
func (h *ConversationHistory) AddMessage(role, content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, Message{Role: role, Content: content})

	// If we exceed max rolling window, drop the oldest messages to preserve context budget
	if len(h.messages) > h.maxLen {
		h.messages = h.messages[len(h.messages)-h.maxLen:]
	}
}

// GetMessages returns a safe copy of the current rolling history.
func (h *ConversationHistory) GetMessages() []Message {
	h.mu.Lock()
	defer h.mu.Unlock()

	copyMsgs := make([]Message, len(h.messages))
	copy(copyMsgs, h.messages)
	return copyMsgs
}

// Clear removes all history for a chat.
func ClearHistory(chatID int64) {
	memCache.Delete(chatID)
}
