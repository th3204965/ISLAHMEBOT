package function

import (
	"net/http"

	"github.com/user/islahmebot/telegram"
)

// MainHandler is the Google Cloud Function entry point.
// Since Go 1.11+, cloud functions use the standard http.HandlerFunc signature.
func MainHandler(w http.ResponseWriter, r *http.Request) {
	telegram.HandleWebhook(w, r)
}
