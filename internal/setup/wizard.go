package setup

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"golang.org/x/term"
)

func NeedsSetup(database *sql.DB) bool {
	_, err := db.GetConfig(database)
	return err != nil
}

func RunWizard(database *sql.DB) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("Welcome to flightdeck.")
	fmt.Println("Let's get you set up.")
	fmt.Println()

	if err := checkAndInstallDeps(reader); err != nil {
		return err
	}

	fmt.Println()

	fmt.Print("Admin username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read username: %w", err)
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	fmt.Print("Admin password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	fmt.Println()
	password := string(passwordBytes)
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	fmt.Print("Confirm password: ")
	confirmBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	fmt.Println()
	if string(confirmBytes) != password {
		return fmt.Errorf("passwords do not match")
	}

	fmt.Print("Control panel domain (leave blank to use IP only): ")
	domain, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read domain: %w", err)
	}
	domain = strings.TrimSpace(domain)

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
	if domain != "" {
		cfg.PanelDomain = sql.NullString{String: domain, Valid: true}
	}

	if err := db.InsertConfig(database, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	if domain != "" {
		fmt.Printf("  Domain configured: https://%s\n", domain)
	} else {
		fmt.Println("  Skipping domain setup. Access at: http://<your-ip>:3000")
	}
	fmt.Println()
	fmt.Println("Setup complete. flightdeck is running.")
	fmt.Println()

	return nil
}

func checkAndInstallDeps(reader *bufio.Reader) error {
	fmt.Println("Checking dependencies...")
	fmt.Println()

	git := checkDep("Git", "git", "--version")
	caddy := checkDep("Caddy", "caddy", "version")

	if git.installed {
		fmt.Printf("  ✓ git %s\n", git.version)
	}
	if caddy.installed {
		fmt.Printf("  ✓ caddy %s\n", caddy.version)
	}

	if git.installed && caddy.installed {
		return nil
	}

	fmt.Println()

	if !git.installed {
		fmt.Print("  Git is required. Install now? [Y/n] ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "" && answer != "y" && answer != "yes" {
			return fmt.Errorf("git is required — install it manually and try again")
		}
		fmt.Println("  Installing git...")
		if err := installGit(); err != nil {
			return fmt.Errorf("failed to install git: %w", err)
		}
		fmt.Println("  ✓ git installed")
	}

	if !caddy.installed {
		fmt.Print("  Caddy is required for SSL and domain routing. Install now? [Y/n] ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "" && answer != "y" && answer != "yes" {
			return fmt.Errorf("caddy is required — install it manually and try again")
		}
		fmt.Println("  Installing caddy...")
		if err := installCaddy(); err != nil {
			return fmt.Errorf("failed to install caddy: %w", err)
		}
	}

	return nil
}
