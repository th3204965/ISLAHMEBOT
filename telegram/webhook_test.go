package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWebhook_MethodNotAllowed(t *testing.T) {
	rr := httptest.NewRecorder()
	HandleWebhook(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("bad"))
	HandleWebhook(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleWebhook_NoMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"update_id":1}`))
	HandleWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhook_MessageWithoutVoice(t *testing.T) {
	body := `{"update_id":1,"message":{"message_id":1,"chat":{"id":456}}}`
	rr := httptest.NewRecorder()
	HandleWebhook(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestUpdateModel_JSONParsing(t *testing.T) {
	j := `{"update_id":999,"message":{"message_id":1,"chat":{"id":12345},"voice":{"file_id":"abc","duration":5}}}`
	var u Update
	if err := json.Unmarshal([]byte(j), &u); err != nil {
		t.Fatal(err)
	}
	if u.Message == nil || u.Message.Voice == nil {
		t.Fatal("message/voice should not be nil")
	}
	if u.Message.Voice.FileID != "abc" || u.Message.Chat.ID != 12345 {
		t.Error("unexpected values")
	}
}

func TestFileResponse_JSONParsing(t *testing.T) {
	j := `{"ok":true,"result":{"file_path":"voice/file.ogg"}}`
	var r FileResponse
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Ok || r.Result.FilePath != "voice/file.ogg" {
		t.Error("unexpected values")
	}
}

func TestFileResponse_NotOk(t *testing.T) {
	j := `{"ok":false,"result":{"file_path":""}}`
	var r FileResponse
	json.Unmarshal([]byte(j), &r)
	if r.Ok {
		t.Error("expected not ok")
	}
}
