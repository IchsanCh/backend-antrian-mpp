package models

import "time"

// Audio - Model untuk tts_audio_cache
type Audio struct {
	ID        int64     `json:"id"`
	TTSText   string    `json:"tts_text"`
	NamaAudio string    `json:"nama_audio"`
	PathAudio string    `json:"path_audio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}