package handler

import (
	"backend-antrian/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
)

// VisitorReportData represents data for each row in the report
type VisitorReportData struct {
	No         int
	Name       string
	DateCounts map[string]int // key: date (DD/MM/YY), value: count
	Total      int
}

// ServiceByUnit represents services grouped by unit
type ServiceByUnit struct {
	UnitID      int64
	UnitName    string
	Services    []VisitorReportData
}

// ExportVisitorReport generates and downloads visitor report in Excel format
func ExportVisitorReport(c *fiber.Ctx) error {
	// Parse query parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	includeServices := c.Query("include_services", "false") == "true"

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
	dateColumns := generateDateColumns(startDate, endDate)

	// Get unit report data
	unitReportData, err := getUnitReportData(startDate, endDate, dateColumns)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate unit report: " + err.Error(),
		})
	}

	// Get service report data if requested (grouped by unit)
	var servicesByUnit []ServiceByUnit
	if includeServices {
		servicesByUnit, err = getServiceReportDataGroupedByUnit(startDate, endDate, dateColumns)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate service report: " + err.Error(),
			})
		}
	}

	// Generate HTML table
	htmlContent := generateHTMLReport(unitReportData, servicesByUnit, dateColumns, includeServices)

	// Create temporary file
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("laporan_kunjungan_%s.xls", timestamp)
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
func generateDateColumns(start, end time.Time) []string {
	var dates []string
	current := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	for !current.After(endDay) {
		dates = append(dates, current.Format("02/01/06"))
		current = current.AddDate(0, 0, 1)
	}

	return dates
}

// getUnitReportData retrieves visitor count data grouped by unit
func getUnitReportData(startDate, endDate time.Time, dateColumns []string) ([]VisitorReportData, error) {
	db := config.DB

	// Query to get all units (tanpa filter is_active)
	type Unit struct {
		ID       int64
		NamaUnit string
	}

	var units []Unit
	query := `SELECT id, nama_unit FROM units ORDER BY nama_unit`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var unit Unit
		if err := rows.Scan(&unit.ID, &unit.NamaUnit); err != nil {
			continue
		}
		units = append(units, unit)
	}

	var reportData []VisitorReportData

	for idx, unit := range units {
		data := VisitorReportData{
			No:         idx + 1,
			Name:       unit.NamaUnit,
			DateCounts: make(map[string]int),
			Total:      0,
		}

		// Get visitor counts per date for this unit
		for _, dateStr := range dateColumns {
			// Parse date string back to time.Time for querying
			date, _ := time.Parse("02/01/06", dateStr)
			dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
			dayEnd := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, date.Location())

			var count int
			countQuery := `
				SELECT COUNT(*) 
				FROM queue_tickets 
				WHERE unit_id = ? 
				AND created_at >= ? 
				AND created_at <= ?
			`
			if err := db.QueryRow(countQuery, unit.ID, dayStart, dayEnd).Scan(&count); err != nil {
				return nil, err
			}

			data.DateCounts[dateStr] = count
			data.Total += count
		}

		reportData = append(reportData, data)
	}

	return reportData, nil
}

// getServiceReportDataGroupedByUnit retrieves visitor count data grouped by unit and their services
func getServiceReportDataGroupedByUnit(startDate, endDate time.Time, dateColumns []string) ([]ServiceByUnit, error) {
	db := config.DB

	// Query to get all units (tanpa filter is_active)
	type Unit struct {
		ID       int64
		NamaUnit string
	}

	var units []Unit
	unitQuery := `SELECT id, nama_unit FROM units ORDER BY nama_unit`
	
	rows, err := db.Query(unitQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var unit Unit
		if err := rows.Scan(&unit.ID, &unit.NamaUnit); err != nil {
			continue
		}
		units = append(units, unit)
	}

	var servicesByUnit []ServiceByUnit

	// For each unit, get its services
	for _, unit := range units {
		serviceByUnit := ServiceByUnit{
			UnitID:   unit.ID,
			UnitName: unit.NamaUnit,
			Services: []VisitorReportData{},
		}

		// Query to get services for this unit (tanpa filter is_active)
		type Service struct {
			ID          int64
			NamaService string
		}

		var services []Service
		serviceQuery := `
			SELECT id, nama_service 
			FROM services 
			WHERE unit_id = ?
			ORDER BY nama_service
		`
		
		serviceRows, err := db.Query(serviceQuery, unit.ID)
		if err != nil {
			return nil, err
		}

		for serviceRows.Next() {
			var service Service
			if err := serviceRows.Scan(&service.ID, &service.NamaService); err != nil {
				continue
			}
			services = append(services, service)
		}
		serviceRows.Close()

		// Get visitor counts for each service
		for _, service := range services {
			data := VisitorReportData{
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

			serviceByUnit.Services = append(serviceByUnit.Services, data)
		}

		// Only add unit if it has services
		if len(serviceByUnit.Services) > 0 {
			servicesByUnit = append(servicesByUnit, serviceByUnit)
		}
	}

	return servicesByUnit, nil
}

// generateHTMLReport creates HTML table for Excel export
func generateHTMLReport(unitData []VisitorReportData, servicesByUnit []ServiceByUnit, dateColumns []string, includeServices bool) string {
	html := "<table border='1'>"

	// Unit Report Section
	html += generateUnitReportSection(unitData, dateColumns)

	// Service Report Section (if requested)
	if includeServices && len(servicesByUnit) > 0 {
		html += "<tr><td colspan='" + fmt.Sprintf("%d", len(dateColumns)+3) + "'>&nbsp;</td></tr>"
		html += generateServiceReportSection(servicesByUnit, dateColumns)
	}

	html += "</table>"
	return html
}

// generateUnitReportSection creates unit report section with header and data
func generateUnitReportSection(data []VisitorReportData, dateColumns []string) string {
	html := ""

	// Title row
	colSpan := len(dateColumns) + 3 // No + Name + Dates + Total
	html += fmt.Sprintf("<tr><th colspan='%d'><h3>Laporan Kunjungan MPP Kabupaten Pekalongan</h3></th></tr>", colSpan)

	// Header row
	html += "<tr><th>No</th><th>Nama Instansi</th>"
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

// generateServiceReportSection creates service report section grouped by unit
func generateServiceReportSection(servicesByUnit []ServiceByUnit, dateColumns []string) string {
	html := ""

	// Title row
	colSpan := len(dateColumns) + 2 // Name + Dates + Total (tanpa kolom No)
	html += fmt.Sprintf("<tr><th colspan='%d'><h3>Laporan Kunjungan Per Layanan</h3></th></tr>", colSpan)

	// Header row
	html += "<tr><th>Nama Layanan</th>"
	for _, date := range dateColumns {
		html += "<th>" + date + "</th>"
	}
	html += "<th>Total</th></tr>"

	// Grand total trackers
	grandTotalByDate := make(map[string]int)
	grandTotal := 0

	// Loop through each unit
	for _, unitGroup := range servicesByUnit {
		// Unit header row (spanning all columns)
		html += fmt.Sprintf("<tr><th colspan='%d' style='background-color: #e0e0e0;'>%s</th></tr>", colSpan, unitGroup.UnitName)

		// Services under this unit
		for _, service := range unitGroup.Services {
			html += fmt.Sprintf("<tr><td>%s</td>", service.Name)

			for _, date := range dateColumns {
				count := service.DateCounts[date]
				html += fmt.Sprintf("<td>%d</td>", count)
				grandTotalByDate[date] += count
			}

			html += fmt.Sprintf("<td>%d</td></tr>", service.Total)
			grandTotal += service.Total
		}
	}

	// Grand Total row
	html += "<tr><th>Grand Total</th>"
	for _, date := range dateColumns {
		html += fmt.Sprintf("<th>%d</th>", grandTotalByDate[date])
	}
	html += fmt.Sprintf("<th>%d</th></tr>", grandTotal)

	return html
}