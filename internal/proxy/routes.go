package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const caddyAdminURL = "http://localhost:2019"

func AddRoute(id, domain string, port int) error {
	route := map[string]interface{}{
		"@id":   id,
		"match": []map[string]interface{}{{"host": []string{domain}}},
		"handle": []map[string]interface{}{{
			"handler":   "reverse_proxy",
			"upstreams": []map[string]string{{"dial": fmt.Sprintf("localhost:%d", port)}},
		}},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		caddyAdminURL+"/config/apps/http/servers/srv0/routes",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("add caddy route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("caddy returned %d adding route", resp.StatusCode)
	}
	return nil
}

func RemoveRoute(id string) error {
	req, err := http.NewRequest(http.MethodDelete, caddyAdminURL+"/id/"+id, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("remove caddy route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		return fmt.Errorf("caddy returned %d removing route", resp.StatusCode)
	}
	return nil
}
