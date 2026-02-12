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
	LastCalledAt    *string  `json:"last_called_at"`
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

type ClientInfo struct {
	conn         *websocket.Conn
	writeMux     sync.Mutex
	closeChan    chan struct{}
	closed       bool
	lastPongTime time.Time
	id           string
}

var (
	queueClients   = make(map[*websocket.Conn]*ClientInfo)
	queueMutex     sync.RWMutex
	clientCounter  uint64
	counterMux     sync.Mutex
	cleanupRunning bool
)

/*
|--------------------------------------------------------------------------
| WebSocket Handler
|--------------------------------------------------------------------------
*/

func QueueWebSocket(c *websocket.Conn) {
	// Generate unique client ID
	counterMux.Lock()
	clientCounter++
	clientID := fmt.Sprintf("client-%d", clientCounter)
	counterMux.Unlock()

	client := &ClientInfo{
		conn:         c,
		closeChan:    make(chan struct{}),
		closed:       false,
		lastPongTime: time.Now(),
		id:           clientID,
	}

	log.Printf("[queue] %s connecting from %s", clientID, c.RemoteAddr())
	registerClient(c, client)
	defer unregisterClient(c, clientID)

	// Set ping/pong handlers
	c.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.SetPongHandler(func(string) error {
		client.writeMux.Lock()
		client.lastPongTime = time.Now()
		client.writeMux.Unlock()
		c.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Kirim data awal
	go func() {
		time.Sleep(100 * time.Millisecond)
		broadcastQueueData()
	}()

	// Ping ticker - kirim ping setiap 20 detik
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// Goroutine untuk ping
	go func() {
		for {
			select {
			case <-ticker.C:
				client.writeMux.Lock()
				if client.closed {
					client.writeMux.Unlock()
					return
				}

				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				err := c.WriteMessage(websocket.PingMessage, nil)
				client.writeMux.Unlock()

				if err != nil {
					log.Printf("[queue] %s ping error: %v", clientID, err)
					return
				}
			case <-client.closeChan:
				return
			}
		}
	}()

	// Read loop
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("[queue] %s unexpected close: %v", clientID, err)
			} else {
				log.Printf("[queue] %s closed normally", clientID)
			}
			return
		}
	}
}

func BroadcastQueueUpdate() {
	broadcastQueueData()
}

/*
|--------------------------------------------------------------------------
| Client Management
|--------------------------------------------------------------------------
*/

func registerClient(c *websocket.Conn, client *ClientInfo) {
	queueMutex.Lock()
	queueClients[c] = client
	totalClients := len(queueClients)
	queueMutex.Unlock()
	
	log.Printf("[queue] %s registered, total: %d", client.id, totalClients)

	// Start cleanup goroutine jika belum jalan
	queueMutex.Lock()
	if !cleanupRunning {
		cleanupRunning = true
		go periodicCleanup()
	}
	queueMutex.Unlock()
}

func unregisterClient(c *websocket.Conn, clientID string) {
	queueMutex.Lock()
	client, exists := queueClients[c]
	if exists {
		client.writeMux.Lock()
		if !client.closed {
			client.closed = true
			close(client.closeChan)
		}
		client.writeMux.Unlock()
		delete(queueClients, c)
	}
	totalClients := len(queueClients)
	queueMutex.Unlock()

	_ = c.Close()
	log.Printf("[queue] %s unregistered, total: %d", clientID, totalClients)
}

// Periodic cleanup untuk dead connections
func periodicCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		queueMutex.Lock()
		if len(queueClients) == 0 {
			cleanupRunning = false
			queueMutex.Unlock()
			log.Println("[queue] No clients, stopping cleanup goroutine")
			return
		}
		queueMutex.Unlock()

		now := time.Now()
		var toRemove []*websocket.Conn

		queueMutex.RLock()
		for conn, client := range queueClients {
			client.writeMux.Lock()
			timeSinceLastPong := now.Sub(client.lastPongTime)
			client.writeMux.Unlock()

			// Hapus client yang tidak merespon > 90 detik
			if timeSinceLastPong > 90*time.Second {
				log.Printf("[queue] %s dead (no pong for %v), marking for removal", client.id, timeSinceLastPong)
				toRemove = append(toRemove, conn)
			}
		}
		queueMutex.RUnlock()

		// Remove dead clients
		if len(toRemove) > 0 {
			queueMutex.Lock()
			for _, conn := range toRemove {
				if client, exists := queueClients[conn]; exists {
					client.writeMux.Lock()
					if !client.closed {
						client.closed = true
						close(client.closeChan)
					}
					client.writeMux.Unlock()
					delete(queueClients, conn)
					conn.Close()
					log.Printf("[queue] %s cleaned up", client.id)
				}
			}
			log.Printf("[queue] Cleaned %d dead clients, remaining: %d", len(toRemove), len(queueClients))
			queueMutex.Unlock()
		}
	}
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

	// Snapshot clients
	queueMutex.RLock()
	clients := make([]*ClientInfo, 0, len(queueClients))
	for _, client := range queueClients {
		clients = append(clients, client)
	}
	queueMutex.RUnlock()

	// Broadcast dengan timeout
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func(c *ClientInfo) {
			defer wg.Done()

			c.writeMux.Lock()
			defer c.writeMux.Unlock()

			if c.closed {
				return
			}

			// Set write deadline
			c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[queue] %s write error (will be cleaned): %v", c.id, err)
				c.closed = true
				if c.closeChan != nil {
					select {
					case <-c.closeChan:
						// Already closed
					default:
						close(c.closeChan)
					}
				}

				// Schedule removal
				go func(conn *websocket.Conn, id string) {
					queueMutex.Lock()
					delete(queueClients, conn)
					queueMutex.Unlock()
					conn.Close()
					log.Printf("[queue] %s removed after write error", id)
				}(c.conn, c.id)
			}
		}(client)
	}

	wg.Wait()
}

func findCurrentlyPlaying(queues []QueueData) *QueueData {
	var latest *QueueData
	var latestTime time.Time

	for i := range queues {
		if queues[i].Status == "called" && queues[i].LastCalledAt != nil {
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
		if queues[i].UnitName != queues[j].UnitName {
			return queues[i].UnitName < queues[j].UnitName
		}
		// Sort by ID descending for tickets with same unit
		return queues[i].ID > queues[j].ID
	})
}

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
| Database Query - OPTIMIZED WITH UNION
|--------------------------------------------------------------------------
*/

func getQueueData() ([]QueueData, error) {
	query := `
		SELECT 
			s.id as service_id,
			s.nama_service,
			s.code as service_code,
			s.unit_id,
			u.nama_unit,
			u.main_display,
			u.audio_file,
			COALESCE(qt.id, 0) as ticket_id,
			COALESCE(qt.ticket_code, '-') as ticket_code,
			COALESCE(qt.status, 'waiting') as status,
			qt.last_called_at
		FROM services s
		JOIN units u ON s.unit_id = u.id
		LEFT JOIN (
			-- Get ticket with latest last_called_at per service (regardless of current status)
			SELECT qt1.* 
			FROM queue_tickets qt1
			INNER JOIN (
				SELECT service_id, MAX(last_called_at) as max_called
				FROM queue_tickets
				WHERE last_called_at IS NOT NULL
				  AND DATE(created_at) = CURDATE()
				GROUP BY service_id
			) qt2 ON qt1.service_id = qt2.service_id 
				 AND qt1.last_called_at = qt2.max_called
			
			UNION ALL
			
			-- Get next WAITING ticket per service
			SELECT qt3.* 
			FROM queue_tickets qt3
			INNER JOIN (
				SELECT service_id, MIN(created_at) as min_created
				FROM queue_tickets
				WHERE status = 'waiting' 
				  AND DATE(created_at) = CURDATE()
				GROUP BY service_id
			) qt4 ON qt3.service_id = qt4.service_id 
				 AND qt3.created_at = qt4.min_created
			WHERE qt3.status = 'waiting'
		) qt ON s.id = qt.service_id
		WHERE s.is_active = 'y'
		ORDER BY u.nama_unit ASC, qt.id DESC
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
		audioFile   sql.NullString
		lastCalled  sql.NullTime
	)

	err := rows.Scan(
		&q.ServiceID,
		&q.ServiceName,
		&q.ServiceCode,
		&q.UnitID,
		&q.UnitName,
		&mainDisplay,
		&audioFile,
		&q.ID,
		&q.TicketCode,
		&q.Status,
		&lastCalled,
	)
	if err != nil {
		return q, err
	}

	// Field Loket diisi dari nama_unit untuk response API
	q.Loket = q.UnitName

	if lastCalled.Valid {
		t := lastCalled.Time.Format("2006-01-02 15:04:05")
		q.LastCalledAt = &t
	}

	q.ShouldPlayAudio = mainDisplay == "active" &&
		q.Status == "called" &&
		q.LastCalledAt != nil

	// Generate audio paths menggunakan audio_file dari unit
	audioFileName := ""
	if audioFile.Valid {
		audioFileName = audioFile.String
	}
	q.AudioPaths = generateAudioPaths(q.TicketCode, audioFileName)

	return q, nil
}

func extractNumber(ticketCode string) int {
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
| Audio Path Generation
|--------------------------------------------------------------------------
*/

func generateAudioPaths(ticketCode, audioFile string) []string {
	paths := []string{
		"audio/ting.mp3",
		"audio/nomor_antrian.mp3",
	}

	paths = append(paths, parseTicketCode(ticketCode)...)
	paths = append(paths, "audio/ke_loket.mp3")

	// Gunakan audio_file langsung dari database unit
	if audioFile != "" {
		paths = append(paths, fmt.Sprintf("audio/%s", audioFile))
	}

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