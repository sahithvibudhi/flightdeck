package setup

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/nestops/nestops/internal/auth"
	"github.com/nestops/nestops/internal/db"
	"golang.org/x/crypto/ssh/terminal"
)

func NeedsSetup(database *sql.DB) bool {
	_, err := db.GetConfig(database)
	return err != nil
}

func RunWizard(database *sql.DB) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("Welcome to nestops.")
	fmt.Println("Let's get you set up (takes 30 seconds).")
	fmt.Println()

	// Username
	fmt.Print("Admin username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read username: %w", err)
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Password
	fmt.Print("Admin password: ")
	passwordBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	fmt.Println()
	password := string(passwordBytes)
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Confirm password
	fmt.Print("Confirm password: ")
	confirmBytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	fmt.Println()
	if string(confirmBytes) != password {
		return fmt.Errorf("passwords do not match")
	}

	// Panel domain
	fmt.Print("Control panel domain (leave blank to use IP only): ")
	domain, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read domain: %w", err)
	}
	domain = strings.TrimSpace(domain)

	// Hash password
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	// Generate JWT secret
	secret, err := auth.GenerateSecret()
	if err != nil {
		return err
	}

	// Store config
	cfg := &db.Config{
		AdminUsername: username,
		AdminPassword: hash,
		JWTSecret:     secret,
	}
	if domain != "" {
		cfg.PanelDomain = sql.NullString{String: domain, Valid: true}
	}

	if err := db.InsertConfig(database, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	if domain != "" {
		fmt.Printf("  → Domain configured: https://%s\n", domain)
	} else {
		fmt.Println("  → Skipping domain setup. Access at: http://<your-ip>:3000")
	}
	fmt.Println()
	fmt.Println("Setup complete. nestops is running.")
	fmt.Println()

	return nil
}
