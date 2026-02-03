package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	MaxAudioSize  = 5 * 1024 * 1024 // 5MB
	AudioBasePath = "./public/audio"
)

// GetAllAudios - Ambil semua audio (public endpoint)
func GetAllAudios(c *fiber.Ctx) error {
	query := `
		SELECT id, tts_text, nama_audio, path_audio, created_at, updated_at 
		FROM tts_audio_cache 
		ORDER BY created_at DESC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data audio",
		})
	}
	defer rows.Close()

	audios := []models.Audio{}
	for rows.Next() {
		var audio models.Audio
		err := rows.Scan(
			&audio.ID,
			&audio.TTSText,
			&audio.NamaAudio,
			&audio.PathAudio,
			&audio.CreatedAt,
			&audio.UpdatedAt,
		)
		if err != nil {
			continue
		}
		audios = append(audios, audio)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    audios,
	})
}

// CreateAudio - Upload audio baru (super_user only)
func CreateAudio(c *fiber.Ctx) error {
	// Parse multipart form
	ttsText := c.FormValue("tts_text")
	namaAudio := c.FormValue("nama_audio")

	// Validasi input
	if ttsText == "" || namaAudio == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tts_text dan nama_audio wajib diisi",
		})
	}

	// Normalisasi nama_audio
	namaAudio = strings.ToLower(strings.TrimSpace(namaAudio))
	if !strings.HasSuffix(namaAudio, ".mp3") {
		namaAudio = namaAudio + ".mp3"
	}

	// Validasi nama_audio (hanya huruf, angka, underscore, dash, dan .mp3)
	// Misal: satu.mp3, sepuluh.mp3, dua_belas.mp3
	validName := true
	nameWithoutExt := strings.TrimSuffix(namaAudio, ".mp3")
	for _, char := range nameWithoutExt {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			validName = false
			break
		}
	}

	if !validName {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "nama_audio hanya boleh mengandung huruf kecil, angka, underscore, dan dash",
		})
	}

	// Cek apakah nama_audio sudah ada (unique constraint)
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM tts_audio_cache WHERE nama_audio = ?", namaAudio).Scan(&count)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal validasi nama audio",
		})
	}

	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Nama audio sudah digunakan",
		})
	}

	// Ambil file dari form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File audio wajib diupload",
		})
	}

	// Validasi ukuran file
	if file.Size > MaxAudioSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Ukuran file maksimal %d MB", MaxAudioSize/(1024*1024)),
		})
	}

	// Validasi tipe file (harus MP3)
	if file.Header.Get("Content-Type") != "audio/mpeg" && file.Header.Get("Content-Type") != "audio/mp3" {
		// Double check dari extension
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".mp3" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "File harus berformat MP3",
			})
		}
	}

	// Pastikan direktori audio ada
	if err := os.MkdirAll(AudioBasePath, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat direktori audio",
		})
	}

	// Path tujuan file
	destinationPath := filepath.Join(AudioBasePath, namaAudio)
	pathAudioDB := fmt.Sprintf("public/audio/%s", namaAudio)

	// Buka file upload
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuka file upload",
		})
	}
	defer src.Close()

	// Buat file tujuan
	dst, err := os.Create(destinationPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menyimpan file",
		})
	}
	defer dst.Close()

	// Copy file
	if _, err := io.Copy(dst, src); err != nil {
		// Hapus file jika gagal copy
		os.Remove(destinationPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menyimpan file",
		})
	}

	// Insert ke database
	query := "INSERT INTO tts_audio_cache (tts_text, nama_audio, path_audio) VALUES (?, ?, ?)"
	result, err := config.DB.Exec(query, ttsText, namaAudio, pathAudioDB)
	if err != nil {
		// Hapus file jika gagal insert ke DB
		os.Remove(destinationPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menyimpan data audio ke database",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat
	var audio models.Audio
	config.DB.QueryRow(
		"SELECT id, tts_text, nama_audio, path_audio, created_at, updated_at FROM tts_audio_cache WHERE id = ?",
		id,
	).Scan(&audio.ID, &audio.TTSText, &audio.NamaAudio, &audio.PathAudio, &audio.CreatedAt, &audio.UpdatedAt)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Audio berhasil diupload",
		"data":    audio,
	})
}

// DeleteAudio - Hapus audio berdasarkan ID (super_user only)
func DeleteAudio(c *fiber.Ctx) error {
	id := c.Params("id")

	// Ambil data audio dari database untuk mendapatkan path file
	var audio models.Audio
	query := "SELECT id, tts_text, nama_audio, path_audio, created_at, updated_at FROM tts_audio_cache WHERE id = ?"

	err := config.DB.QueryRow(query, id).Scan(
		&audio.ID,
		&audio.TTSText,
		&audio.NamaAudio,
		&audio.PathAudio,
		&audio.CreatedAt,
		&audio.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Audio tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data audio",
		})
	}

	// Hapus dari database terlebih dahulu
	_, err = config.DB.Exec("DELETE FROM tts_audio_cache WHERE id = ?", id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus data audio dari database",
		})
	}

	// Hapus file fisik
	filePath := filepath.Join(AudioBasePath, audio.NamaAudio)
	if err := os.Remove(filePath); err != nil {
		// Log error tapi tidak return error, karena data sudah terhapus dari DB
		// File mungkin sudah terhapus manual atau tidak ada
		fmt.Printf("Warning: Gagal menghapus file %s: %v\n", filePath, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Audio berhasil dihapus",
	})
}