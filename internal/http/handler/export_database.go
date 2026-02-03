package handler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
)

func ExportDatabase(c *fiber.Ctx) error {
	mode := os.Getenv("BACKUP_MODE")

	switch mode {
	case "windows":
		return ExportDatabaseWindows(c)

	case "docker_exec":
		return ExportDatabaseDockerExec(c)

	case "direct":
		return ExportDatabaseDirect(c)

	default:
		return c.Status(500).JSON(fiber.Map{
			"error": "Invalid BACKUP_MODE",
		})
	}
}

func ExportDatabaseWindows(c *fiber.Ctx) error {
	fileName := fmt.Sprintf("backup-%s.sql", time.Now().Format("20060102-150405"))
	filePath := filepath.Join(os.TempDir(), fileName)
	mysqldumpPath := `D:\laragon\bin\mysql\mysql-8.0.30-winx64\bin\mysqldump.exe`

	cmd := exec.Command(
		mysqldumpPath,
		"--protocol=tcp",
		"--column-statistics=0",
		"--no-tablespaces",           // Fix: Tambah flag ini untuk avoid TABLESPACE errors
		"--skip-add-locks",            // Optional: Skip lock tables
		"--single-transaction",        // Fix: Ensure data consistency
		"--routines",                  // Include stored procedures
		"--triggers",                  // Include triggers
		"--events",                    // Include events
		"--default-character-set=utf8mb4", // Fix: Proper charset handling
		"-h", "127.0.0.1",
		"-P", "3306",
		"-u", os.Getenv("DB_USER"),
		"--password="+os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	file, err := os.Create(filePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer file.Close()
	defer os.Remove(filePath) // Auto cleanup setelah download

	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Fix: Set Content-Disposition header dengan format yang benar
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Set("Content-Type", "application/sql")
	
	return c.SendFile(filePath)
}

func ExportDatabaseDockerExec(c *fiber.Ctx) error {
	fileName := fmt.Sprintf("backup-%s.sql", time.Now().Format("20060102-150405"))
	filePath := "/tmp/" + fileName

	cmd := exec.Command(
		"docker", "exec", "mysql",
		"mysqldump",
		"--no-tablespaces",
		"--single-transaction",
		"--routines",
		"--triggers",
		"--events",
		"--default-character-set=utf8mb4",
		"-u"+os.Getenv("DB_USER"),
		"--password="+os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	file, err := os.Create(filePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer file.Close()
	defer os.Remove(filePath)

	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Set("Content-Type", "application/sql")
	
	return c.SendFile(filePath)
}

func ExportDatabaseDirect(c *fiber.Ctx) error {
	fileName := fmt.Sprintf("backup-%s.sql", time.Now().Format("20060102-150405"))
	filePath := "/tmp/" + fileName

	cmd := exec.Command(
		"mysqldump",
		"--no-tablespaces",
		"--single-transaction",
		"--quick",
		"--routines",
		"--triggers",
		"--events",
		"--default-character-set=utf8mb4",
		"-h", os.Getenv("DB_HOST"),
		"-P", os.Getenv("DB_PORT"),
		"-u"+os.Getenv("DB_USER"),
		"--password="+os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	file, err := os.Create(filePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer file.Close()
	defer os.Remove(filePath)

	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Set("Content-Type", "application/sql")
	
	return c.SendFile(filePath)
}