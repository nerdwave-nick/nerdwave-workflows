package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerNoArgumentsStartsFromConfig(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config.json")
	data := filepath.Join(root, "store")
	content, _ := json.Marshal(map[string]any{"schema_version": 1, "data_dir": data, "listen": "127.0.0.1:0"})
	if err := os.WriteFile(config, content, 0600); err != nil {
		t.Fatal(err)
	}
	log := new(safeBuffer)
	cmd := exec.Command(litBin)
	cmd.Env = append(os.Environ(), "HOME="+root, "USERPROFILE="+root, "XDG_CONFIG_HOME="+filepath.Join(root, "config"), "LIT_CONFIG_FILE="+config, "LIT_DATA_DIR=", "LIT_LISTEN=", "GORACE=atexit_sleep_ms=0")
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	exited := false
	t.Cleanup(func() {
		if exited {
			return
		}
		cmd.Process.Signal(os.Interrupt)
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("shutdown: %v: %s", err, log.String())
			}
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-done
			t.Error("server did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			exited = true
			t.Fatalf("server exited before listening: %v: %s", err, log.String())
		default:
		}
		if _, after, ok := strings.Cut(log.String(), "lit-server listening "); ok {
			fields := strings.Fields(after)
			if len(fields) > 0 {
				client := http.Client{Timeout: time.Second}
				response, err := client.Get("http://" + fields[0] + "/v1/meta")
				if err == nil {
					response.Body.Close()
					if response.StatusCode != http.StatusOK {
						t.Fatalf("meta status %d", response.StatusCode)
					}
					if _, err := os.Stat(data); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no listener: %s", log.String())
}
