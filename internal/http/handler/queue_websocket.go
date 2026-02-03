package handler

import (
	"backend-antrian/internal/config"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

/*
|--------------------------------------------------------------------------
| Data Structure
|--------------------------------------------------------------------------
*/

type QueueData struct {
	ID              int64    `json:"id"`
	TicketCode      string   `json:"ticket_code"`
	UnitID          int64    `json:"unit_id"`
	UnitName        string   `json:"unit_name"`
	ServiceID       int64    `json:"service_id"`
	ServiceName     string   `json:"service_name"`
	ServiceCode     string   `json:"service_code"`
	Loket           string   `json:"loket"`
	Status          string   `json:"status"`
	ShouldPlayAudio bool     `json:"should_play_audio"`
	AudioPaths      []string `json:"audio_paths"`
	CreatedAt       string   `json:"created_at"`
	LastCalledAt    *string  `json:"last_called_at"`
	TicketNumber    int      `json:"ticket_number"` // untuk sorting
}

type ServiceStats struct {
	WaitingCount int  `json:"waiting_count"`
	HasNext      bool `json:"has_next"`
}

/*
|--------------------------------------------------------------------------
| WebSocket Client Registry
|--------------------------------------------------------------------------
*/

var (
	queueClients = make(map[*websocket.Conn]struct{})
	queueMutex   sync.RWMutex
)

/*
|--------------------------------------------------------------------------
| WebSocket Handler
|--------------------------------------------------------------------------
*/

// QueueWebSocket handles realtime queue websocket connection
func QueueWebSocket(c *websocket.Conn) {
	registerClient(c)
	defer unregisterClient(c)

	// kirim data awal
	broadcastQueueData()

	// keep alive (fiber websocket butuh read loop)
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			return
		}
	}
}

// BroadcastQueueUpdate bisa dipanggil dari handler lain
func BroadcastQueueUpdate() {
	broadcastQueueData()
}

/*
|--------------------------------------------------------------------------
| Client Management
|--------------------------------------------------------------------------
*/

func registerClient(c *websocket.Conn) {
	queueMutex.Lock()
	queueClients[c] = struct{}{}
	queueMutex.Unlock()
}

func unregisterClient(c *websocket.Conn) {
	queueMutex.Lock()
	delete(queueClients, c)
	queueMutex.Unlock()
	_ = c.Close()
}

/*
|--------------------------------------------------------------------------
| Broadcast Logic
|--------------------------------------------------------------------------
*/

func broadcastQueueData() {
	queues, err := getQueueData()
	if err != nil {
		log.Printf("[queue] failed to fetch data: %v", err)
		return
	}

	// Sort berdasarkan unit name A-Z, kemudian ticket number DESC
	sortQueueData(queues)

	currentlyPlaying := findCurrentlyPlaying(queues)
	serviceStats := calculateServiceStats()

	payload := map[string]interface{}{
		"type":              "queue_update",
		"data":              queues,
		"currently_playing": currentlyPlaying,
		"service_stats":     serviceStats,
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	message, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[queue] json marshal error: %v", err)
		return
	}

	queueMutex.RLock()
	defer queueMutex.RUnlock()

	for client := range queueClients {
		if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("[queue] websocket write error: %v", err)
		}
	}
}

func findCurrentlyPlaying(queues []QueueData) *QueueData {
	// Cari yang status 'called' dan punya last_called_at paling baru
	// TANPA cek should_play_audio (biar display utama tetap update meski di-mute)
	var latest *QueueData
	var latestTime time.Time

	for i := range queues {
		if queues[i].Status == "called" && 
		   queues[i].LastCalledAt != nil {  // ✅ Hapus pengecekan ShouldPlayAudio
			
			t, err := time.Parse("2006-01-02 15:04:05", *queues[i].LastCalledAt)
			if err != nil {
				continue
			}

			if latest == nil || t.After(latestTime) {
				latest = &queues[i]
				latestTime = t
			}
		}
	}

	return latest
}

func sortQueueData(queues []QueueData) {
	sort.Slice(queues, func(i, j int) bool {
		// Primary: Unit name A-Z
		if queues[i].UnitName != queues[j].UnitName {
			return queues[i].UnitName < queues[j].UnitName
		}
		// Secondary: Ticket number DESC (angka besar di atas)
		return queues[i].TicketNumber > queues[j].TicketNumber
	})
}

// calculateServiceStats menghitung stats per service untuk caller UI
func calculateServiceStats() map[int64]ServiceStats {
	query := `
		SELECT 
			service_id,
			COUNT(*) as waiting_count
		FROM queue_tickets
		WHERE status = 'waiting'
		  AND DATE(created_at) = CURDATE()
		GROUP BY service_id
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("[queue] failed to calculate service stats: %v", err)
		return make(map[int64]ServiceStats)
	}
	defer rows.Close()

	stats := make(map[int64]ServiceStats)

	for rows.Next() {
		var serviceID int64
		var count int

		if err := rows.Scan(&serviceID, &count); err != nil {
			log.Printf("[queue] scan error in service stats: %v", err)
			continue
		}

		stats[serviceID] = ServiceStats{
			WaitingCount: count,
			HasNext:      count > 0,
		}
	}

	return stats
}

/*
|--------------------------------------------------------------------------
| Database
|--------------------------------------------------------------------------
*/

func getQueueData() ([]QueueData, error) {
	// PERBAIKAN: LEFT JOIN untuk tampilkan semua service yang active
	// meskipun belum ada antrian hari ini
	query := `
		SELECT 
			s.id as service_id,
			s.nama_service,
			s.code as service_code,
			s.loket,
			s.unit_id,
			u.nama_unit,
			u.main_display,
			COALESCE(qt.id, 0) as ticket_id,
			COALESCE(qt.ticket_code, CONCAT(s.code, '000')) as ticket_code,
			COALESCE(qt.status, 'waiting') as status,
			qt.created_at,
			qt.last_called_at
		FROM services s
		JOIN units u ON s.unit_id = u.id
		LEFT JOIN (
			SELECT 
				qt1.*,
				ROW_NUMBER() OVER (
					PARTITION BY qt1.service_id 
					ORDER BY qt1.last_called_at DESC, qt1.created_at DESC
				) as rn
			FROM queue_tickets qt1
			WHERE qt1.status IN ('waiting', 'called')
			  AND DATE(qt1.created_at) = CURDATE()
		) qt ON s.id = qt.service_id AND qt.rn = 1
		WHERE s.is_active = 'y'
		ORDER BY u.nama_unit ASC, qt.created_at DESC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []QueueData

	for rows.Next() {
		queue, err := scanQueueRow(rows)
		if err != nil {
			log.Printf("[queue] scan error: %v", err)
			continue
		}
		result = append(result, queue)
	}

	return result, nil
}

func scanQueueRow(rows *sql.Rows) (QueueData, error) {
	var (
		q           QueueData
		mainDisplay string
		createdAt   sql.NullTime
		lastCalled  sql.NullTime
	)

	err := rows.Scan(
		&q.ServiceID,
		&q.ServiceName,
		&q.ServiceCode,
		&q.Loket,
		&q.UnitID,
		&q.UnitName,
		&mainDisplay,
		&q.ID,
		&q.TicketCode,
		&q.Status,
		&createdAt,
		&lastCalled,
	)
	if err != nil {
		return q, err
	}

	// Extract angka dari ticket code untuk sorting
	q.TicketNumber = extractNumber(q.TicketCode)

	if createdAt.Valid {
		q.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05")
	}

	if lastCalled.Valid {
		t := lastCalled.Time.Format("2006-01-02 15:04:05")
		q.LastCalledAt = &t
	}

	// PERBAIKAN: Should play audio hanya jika:
	// 1. main_display = active
	// 2. Status = called (bukan waiting)
	// 3. Ada last_called_at (artinya baru dipanggil)
	q.ShouldPlayAudio = mainDisplay == "active" && 
	                    q.Status == "called" && 
	                    q.LastCalledAt != nil

	q.AudioPaths = generateAudioPaths(q.TicketCode, q.Loket)

	return q, nil
}

func todayRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return start, start.Add(24 * time.Hour)
}

func extractNumber(ticketCode string) int {
	// Extract angka dari ticket code (misal AK004 -> 4)
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(ticketCode)
	if match == "" {
		return 0
	}
	num, _ := strconv.Atoi(match)
	return num
}

/*
|--------------------------------------------------------------------------
| Audio Logic
|--------------------------------------------------------------------------
*/

func generateAudioPaths(ticketCode, loket string) []string {
	paths := []string{
		"audio/ting.mp3",
		"audio/nomor_antrian.mp3",
	}

	paths = append(paths, parseTicketCode(ticketCode)...)
	paths = append(paths, "audio/silahkan_menuju_loket.mp3")
	paths = append(paths, parseLoket(loket)...)

	return paths
}

func parseTicketCode(code string) []string {
	var letters, numbers string

	for _, c := range code {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
			letters += string(c)
		case c >= '0' && c <= '9':
			numbers += string(c)
		}
	}

	var paths []string

	for _, c := range strings.ToLower(letters) {
		paths = append(paths, fmt.Sprintf("audio/%c.mp3", c))
	}

	if numbers != "" {
		if n, err := strconv.Atoi(numbers); err == nil {
			paths = append(paths, parseNumberToAudio(n)...)
		}
	}

	return paths
}

func parseLoket(loket string) []string {
	if n, err := strconv.Atoi(loket); err == nil {
		return parseNumberToAudio(n)
	}
	return []string{fmt.Sprintf("audio/%s.mp3", loket)}
}

/*
|--------------------------------------------------------------------------
| Number to Audio (Bahasa Indonesia)
|--------------------------------------------------------------------------
*/

func parseNumberToAudio(num int) []string {
	if num == 0 {
		return []string{"audio/nol.mp3"}
	}

	ones := []string{
		"", "satu", "dua", "tiga", "empat",
		"lima", "enam", "tujuh", "delapan", "sembilan",
	}

	switch {
	case num < 10:
		return []string{fmt.Sprintf("audio/%s.mp3", ones[num])}

	case num == 10:
		return []string{"audio/sepuluh.mp3"}

	case num == 11:
		return []string{"audio/sebelas.mp3"}

	case num < 20:
		return []string{
			fmt.Sprintf("audio/%s.mp3", ones[num-10]),
			"audio/belas.mp3",
		}

	case num < 100:
		res := []string{
			fmt.Sprintf("audio/%s.mp3", ones[num/10]),
			"audio/puluh.mp3",
		}
		if num%10 > 0 {
			res = append(res, fmt.Sprintf("audio/%s.mp3", ones[num%10]))
		}
		return res

	case num == 100:
		return []string{"audio/seratus.mp3"}

	case num < 200:
		return append(
			[]string{"audio/seratus.mp3"},
			parseNumberToAudio(num-100)...,
		)

	case num < 1000:
		res := []string{
			fmt.Sprintf("audio/%s.mp3", ones[num/100]),
			"audio/ratus.mp3",
		}
		if num%100 > 0 {
			res = append(res, parseNumberToAudio(num%100)...)
		}
		return res

	case num == 1000:
		return []string{"audio/seribu.mp3"}
	}

	return nil
}