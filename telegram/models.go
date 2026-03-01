package telegram

type Update struct {
	UpdateID int     `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Voice     *Voice `json:"voice,omitempty"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Voice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
}

type FileResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		FilePath string `json:"file_path"`
	} `json:"result"`
}
