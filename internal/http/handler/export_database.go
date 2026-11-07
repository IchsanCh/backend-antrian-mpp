package handler

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	case "lamp":
		return ExportDatabaseLamp(c)

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

// findDumpBinary cari binary dump yang tersedia di server LAMP.
// Ubuntu + MariaDB -> mariadb-dump (10.5+), mysqldump = symlink (deprecated mulai 11.0)
// Ubuntu + MySQL   -> mysqldump
func findDumpBinary() (string, error) {
	for _, name := range []string{"mariadb-dump", "mysqldump"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("mariadb-dump / mysqldump tidak ketemu di PATH (install: sudo apt install mariadb-client)")
}

// ExportDatabaseLamp: backup untuk LAMP stack di Ubuntu (MariaDB atau MySQL native).
// Cuma pakai flag yang didukung mariadb-dump DAN mysqldump, jadi aman di dua engine.
// Sengaja TIDAK pakai --column-statistics=0 (cuma ada di mysqldump 8, bikin error di MariaDB).
func ExportDatabaseLamp(c *fiber.Ctx) error {
	bin, err := findDumpBinary()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}

	fileName := fmt.Sprintf("backup-%s.sql", time.Now().Format("20060102-150405"))
	filePath := filepath.Join(os.TempDir(), fileName)

	cmd := exec.Command(
		bin,
		"--protocol=tcp",
		"--no-tablespaces",
		"--single-transaction",
		"--quick",
		"--routines",
		"--triggers",
		"--events",
		"--default-character-set=utf8mb4",
		"-h", host,
		"-P", port,
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

	var stderr bytes.Buffer
	cmd.Stdout = file
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		log.Printf("[BACKUP][lamp] %s gagal: %v - %s", bin, err, msg)
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "detail": msg})
	}

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Set("Content-Type", "application/sql")

	return c.SendFile(filePath)
}