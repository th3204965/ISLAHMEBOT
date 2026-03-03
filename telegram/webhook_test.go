package telegram

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleWebhook_MethodNotAllowed(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/webhook", nil)
	rr := httptest.NewRecorder()

	HandleWebhook(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
	}
}

func TestHandleWebhook_BadRequest(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString("{invalid json"))
	rr := httptest.NewRecorder()

	HandleWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rr.Code)
	}
}

func TestHandleWebhook_NoVoiceMessage(t *testing.T) {
	// A valid update but no voice message
	updateJSON := `{"update_id": 123, "message": {"message_id": 1, "text": "hello"}}`
	req, _ := http.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(updateJSON))
	rr := httptest.NewRecorder()

	HandleWebhook(rr, req)

	// Should just return 200 OK and ignore
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}
}
