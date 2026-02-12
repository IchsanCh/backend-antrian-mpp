package handler

import (
	"backend-antrian/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// VisitorReportData represents data for each row in the report
type VisitorReportDataUnit struct {
	No         int
	Name       string
	DateCounts map[string]int // key: date (DD/MM/YY), value: count
	Total      int
}

// ExportUnitVisitorReport generates and downloads visitor report for specific unit (unit role)
func ExportUnitVisitorReport(c *fiber.Ctx) error {
	// Get unit_id from JWT token
	unitID, ok := c.Locals("unit_id").(int64)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Unit ID not found in token",
		})
	}

	// Parse query parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	// Validate required parameters
	if startDateStr == "" || endDateStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "start_date and end_date are required",
		})
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid start_date format. Use YYYY-MM-DD",
		})
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid end_date format. Use YYYY-MM-DD",
		})
	}

	// Set time to cover full day range
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 1, 0, startDate.Location())
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, endDate.Location())

	// Generate date columns
	dateColumns := generateDateColumnsUnit(startDate, endDate)

	// Get unit name
	db := config.DB
	var unitName string
	err = db.QueryRow("SELECT nama_unit FROM units WHERE id = ?", unitID).Scan(&unitName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get unit information: " + err.Error(),
		})
	}

	// Get service report data for this unit only
	serviceReportData, err := getUnitServiceReportData(unitID, startDate, endDate, dateColumns)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate service report: " + err.Error(),
		})
	}

	// Generate HTML table
	htmlContent := generateUnitHTMLReport(unitName, serviceReportData, dateColumns)

	// Create temporary file
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("laporan_kunjungan_%s_%s.xls", sanitizeFilename(unitName), timestamp)
	tempDir := "./temp"
	
	// Create temp directory if not exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create temp directory",
		})
	}

	filePath := filepath.Join(tempDir, filename)

	// Write HTML content to file
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create file",
		})
	}

	// Set headers for download
	c.Set("Content-Type", "application/vnd.ms-excel")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Send file
	err = c.SendFile(filePath)

	// Schedule cleanup with retry
	go func(path string) {
		// Try to delete with retries
		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			time.Sleep(10 * time.Second) 
			
			if err := os.Remove(path); err == nil {
				fmt.Printf("Successfully deleted temp file: %s\n", path)
				return
			} else if i == maxRetries-1 {
				fmt.Printf("Failed to delete after %d retries: %s - %v\n", maxRetries, path, err)
			}
		}
	}(filePath)

	return err
}

// generateDateColumns creates a slice of dates between start and end
func generateDateColumnsUnit(start, end time.Time) []string {
	var dates []string
	current := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	for !current.After(endDay) {
		dates = append(dates, current.Format("02/01/06"))
		current = current.AddDate(0, 0, 1)
	}

	return dates
}

// getUnitServiceReportData retrieves visitor count data for services of a specific unit
func getUnitServiceReportData(unitID int64, startDate, endDate time.Time, dateColumns []string) ([]VisitorReportData, error) {
	db := config.DB

	// Query to get active services from this unit only
	type Service struct {
		ID          int64
		NamaService string
	}

	var services []Service
	query := `
		SELECT s.id, s.nama_service 
		FROM services s
		WHERE s.unit_id = ? 
		ORDER BY s.nama_service
	`
	
	rows, err := db.Query(query, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var service Service
		if err := rows.Scan(&service.ID, &service.NamaService); err != nil {
			continue
		}
		services = append(services, service)
	}

	var reportData []VisitorReportData

	for idx, service := range services {
		data := VisitorReportData{
			No:         idx + 1,
			Name:       service.NamaService,
			DateCounts: make(map[string]int),
			Total:      0,
		}

		// Get visitor counts per date for this service
		for _, dateStr := range dateColumns {
			// Parse date string back to time.Time for querying
			date, _ := time.Parse("02/01/06", dateStr)
			dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
			dayEnd := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, date.Location())

			var count int
			countQuery := `
				SELECT COUNT(*) 
				FROM queue_tickets 
				WHERE service_id = ? 
				AND created_at >= ? 
				AND created_at <= ?
			`
			if err := db.QueryRow(countQuery, service.ID, dayStart, dayEnd).Scan(&count); err != nil {
				return nil, err
			}

			data.DateCounts[dateStr] = count
			data.Total += count
		}

		reportData = append(reportData, data)
	}

	return reportData, nil
}

// generateHTMLReport creates HTML table for Excel export (Super User)
func generateHTMLReportUnit(unitData, serviceData []VisitorReportData, dateColumns []string, includeServices bool) string {
	html := "<table border='1'>"

	// Unit Report Section
	html += generateReportSectionUnit("Laporan Kunjungan MPP Kabupaten Pekalongan", "Nama Instansi", unitData, dateColumns)

	// Service Report Section (if requested)
	if includeServices && len(serviceData) > 0 {
		html += "<tr><td colspan='" + fmt.Sprintf("%d", len(dateColumns)+3) + "'>&nbsp;</td></tr>"
		html += generateReportSectionUnit("Laporan Kunjungan Per Layanan", "Nama Layanan", serviceData, dateColumns)
	}

	html += "</table>"
	return html
}

// generateUnitHTMLReport creates HTML table for unit's Excel export
func generateUnitHTMLReport(unitName string, serviceData []VisitorReportData, dateColumns []string) string {
	html := "<table border='1'>"

	// Service Report Section
	title := fmt.Sprintf("Laporan Kunjungan %s", unitName)
	html += generateReportSectionUnit(title, "Nama Layanan", serviceData, dateColumns)

	html += "</table>"
	return html
}

// generateReportSection creates a report section with header and data
func generateReportSectionUnit(title, nameColumn string, data []VisitorReportData, dateColumns []string) string {
	html := ""

	// Title row
	colSpan := len(dateColumns) + 3 // No + Name + Dates + Total
	html += fmt.Sprintf("<tr><th colspan='%d'><h3>%s</h3></th></tr>", colSpan, title)

	// Header row
	html += "<tr><th>No</th><th>" + nameColumn + "</th>"
	for _, date := range dateColumns {
		html += "<th>" + date + "</th>"
	}
	html += "<th>Total</th></tr>"

	// Data rows
	grandTotalByDate := make(map[string]int)
	grandTotal := 0

	for _, row := range data {
		html += fmt.Sprintf("<tr><td>%d</td><td>%s</td>", row.No, row.Name)

		for _, date := range dateColumns {
			count := row.DateCounts[date]
			html += fmt.Sprintf("<td>%d</td>", count)
			grandTotalByDate[date] += count
		}

		html += fmt.Sprintf("<td>%d</td></tr>", row.Total)
		grandTotal += row.Total
	}

	// Grand Total row
	html += "<tr><th colspan='2'>Grand Total</th>"
	for _, date := range dateColumns {
		html += fmt.Sprintf("<th>%d</th>", grandTotalByDate[date])
	}
	html += fmt.Sprintf("<th>%d</th></tr>", grandTotal)

	return html
}

// sanitizeFilename removes invalid characters from filename
func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscore
	name = strings.TrimSpace(name)
	result := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			result += string(char)
		} else if char == ' ' || char == '-' {
			result += "_"
		}
	}
	// Avoid empty result
	if result == "" {
		result = "unit"
	}
	return result
}