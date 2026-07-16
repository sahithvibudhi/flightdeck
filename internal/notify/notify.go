package notify

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sahithvibudhi/flightdeck/internal/db"
)

var client = &http.Client{Timeout: 5 * time.Second}

// telegramBase is a variable so tests can point it at a local server.
var telegramBase = "https://api.telegram.org"

/*
Send posts an event to every configured channel. Errors are returned
joined so the test endpoint can show them; deploy and crash hooks call
Go(), which fires in the background and only logs failures.
*/
func Send(database *sql.DB, title, message string) error {
	cfg, err := db.GetConfig(database)
	if err != nil {
		return err
	}

	var errs []string

	if cfg.NotifyDiscord != "" {
		if err := postJSON(cfg.NotifyDiscord, map[string]string{
			"content": "**" + title + "**\n" + message,
		}); err != nil {
			errs = append(errs, "discord: "+err.Error())
		}
	}

	if cfg.NotifyTelegramToken != "" && cfg.NotifyTelegramChat != "" {
		url := fmt.Sprintf("%s/bot%s/sendMessage", telegramBase, cfg.NotifyTelegramToken)
		if err := postJSON(url, map[string]string{
			"chat_id": cfg.NotifyTelegramChat,
			"text":    title + "\n" + message,
		}); err != nil {
			errs = append(errs, "telegram: "+err.Error())
		}
	}

	if cfg.NotifyWebhook != "" {
		if err := postJSON(cfg.NotifyWebhook, map[string]string{
			"title":     title,
			"message":   message,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			errs = append(errs, "webhook: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// Go sends in the background; deploys must never block on a slow channel.
func Go(database *sql.DB, title, message string) {
	go func() {
		if err := Send(database, title, message); err != nil {
			log.Printf("notification failed: %v", err)
		}
	}()
}

// Configured reports whether any channel is set, so callers can skip
// building messages nobody will receive.
func Configured(database *sql.DB) bool {
	cfg, err := db.GetConfig(database)
	if err != nil {
		return false
	}
	return cfg.NotifyDiscord != "" ||
		(cfg.NotifyTelegramToken != "" && cfg.NotifyTelegramChat != "") ||
		cfg.NotifyWebhook != ""
}

func postJSON(url string, payload map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("returned status %d", resp.StatusCode)
	}
	return nil
}
