package integration

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// activated starts lit-server the way systemd socket activation does: the
// listening socket is inherited as fd 3 with LISTEN_PID naming the server's own
// PID (the shell execs in place, so $$ is the server's PID). The test's copies
// of the socket are closed, so only the server can accept on it.
func activated(t *testing.T, network, addr, fds string, out *safeBuffer, args ...string) (*exec.Cmd, string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("socket activation is a Linux service feature")
	}
	l, e := net.Listen(network, addr)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	f, e := l.(*net.TCPListener).File()
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	c := exec.Command("sh", append([]string{"-c", `LISTEN_PID=$$ exec "$0" "$@"`, litBin}, args...)...)
	c.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0", "LISTEN_FDS="+fds, "LISTEN_FDNAMES=lit.socket")
	c.ExtraFiles = []*os.File{f}
	c.Stdout, c.Stderr = out, out
	if e = c.Start(); e != nil {
		t.Fatal(e)
	}
	return c, l.Addr().String()
}

func TestSocketActivationServesInheritedListener(t *testing.T) {
	log := new(safeBuffer)
	// The configured listener is deliberately different; systemd owns the address.
	c, addr := activated(t, "tcp", "127.0.0.1:0", "1", log, "--data-dir", filepath.Join(t.TempDir(), "data"), "--listen", "127.0.0.1:1")
	s := &server{c, "http://" + addr, log}
	defer s.stop(t)
	client := http.Client{Timeout: time.Second}
	for i := 0; i < 200; i++ {
		r, e := client.Get(s.endpoint + "/v1/meta")
		if e == nil {
			r.Body.Close()
			if r.StatusCode != http.StatusOK {
				t.Fatalf("meta status %d", r.StatusCode)
			}
			if !strings.Contains(log.String(), "lit-server listening "+addr+" (socket-activated)") {
				t.Fatalf("log does not report inherited listener: %s", log)
			}
			return
		}
		if c.ProcessState != nil || strings.Contains(log.String(), "cannot listen") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("inherited listener unavailable: %s", log)
}

func TestSocketActivationRejectsUnsupportedSockets(t *testing.T) {
	for _, tc := range []struct{ name, addr, fds, want string }{
		{"non-loopback", "0.0.0.0:0", "1", "loopback"},
		{"multiple", "127.0.0.1:0", "2", "exactly one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := new(safeBuffer)
			// A free configured port keeps a fallback bind from touching a real service.
			c, _ := activated(t, "tcp", tc.addr, tc.fds, out, "--data-dir", filepath.Join(t.TempDir(), "data"), "--listen", "127.0.0.1:0")
			done := make(chan error, 1)
			go func() { done <- c.Wait() }()
			select {
			case e := <-done:
				if e == nil || !strings.Contains(out.String(), tc.want) {
					t.Fatalf("exit %v, want rejection containing %q: %s", e, tc.want, out)
				}
			case <-time.After(5 * time.Second):
				c.Process.Kill()
				<-done
				t.Fatalf("server accepted unsupported socket: %s", out)
			}
		})
	}
}
