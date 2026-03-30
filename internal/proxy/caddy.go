package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Caddy struct {
	cmd     *exec.Cmd
	dataDir string
}

func NewCaddy(dataDir string) *Caddy {
	return &Caddy{dataDir: dataDir}
}

func (c *Caddy) Start() error {
	configPath := filepath.Join(c.dataDir, "caddy", "caddy.json")

	if err := c.writeDefaultConfig(configPath); err != nil {
		return err
	}

	c.cmd = exec.Command("caddy", "run", "--config", configPath)
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start caddy: %w", err)
	}

	go func() {
		c.cmd.Wait()
	}()

	return nil
}

func (c *Caddy) Stop() {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Signal(os.Interrupt)
		c.cmd.Wait()
	}
}

func (c *Caddy) writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	config := map[string]interface{}{
		"admin": map[string]interface{}{
			"listen": "localhost:2019",
		},
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"srv0": map[string]interface{}{
						"listen": []string{":80", ":443"},
						"routes": []interface{}{},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
