package setup

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

/*
CreateConfig validates the admin credentials, generates a JWT secret,
and persists the initial config. It is shared by the terminal wizard,
the web setup endpoint, and env-based headless provisioning.
*/
func CreateConfig(database *sql.DB, username, password, domain string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	secret, err := auth.GenerateSecret()
	if err != nil {
		return err
	}

	cfg := &db.Config{
		AdminUsername: username,
		AdminPassword: hash,
		JWTSecret:     secret,
	}
	if domain = strings.TrimSpace(domain); domain != "" {
		cfg.PanelDomain = sql.NullString{String: domain, Valid: true}
	}

	if err := db.InsertConfig(database, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

/*
SeedFromEnv creates the admin account from FLIGHTDECK_ADMIN_USER and
FLIGHTDECK_ADMIN_PASSWORD so automated provisioning (cloud-init, Ansible)
can skip interactive setup entirely. Returns true if config was created.
*/
func SeedFromEnv(database *sql.DB) (bool, error) {
	username := os.Getenv("FLIGHTDECK_ADMIN_USER")
	password := os.Getenv("FLIGHTDECK_ADMIN_PASSWORD")
	if username == "" || password == "" {
		return false, nil
	}
	if err := CreateConfig(database, username, password, os.Getenv("FLIGHTDECK_PANEL_DOMAIN")); err != nil {
		return false, fmt.Errorf("seed admin from environment: %w", err)
	}
	return true, nil
}
