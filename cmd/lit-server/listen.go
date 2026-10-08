package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// listenFDsStart is the first descriptor systemd passes (SD_LISTEN_FDS_START).
const listenFDsStart = 3

// listen returns the socket inherited through systemd socket activation, or binds
// addr when the process was not socket-activated. An inherited socket must still
// be a single loopback TCP listener; the configured address is then unused.
func listen(addr string) (ln net.Listener, activated bool, e error) {
	if os.Getenv("LISTEN_PID") != strconv.Itoa(os.Getpid()) {
		ln, e = net.Listen("tcp", addr)
		return ln, false, e
	}
	n := os.Getenv("LISTEN_FDS")
	for _, k := range []string{"LISTEN_PID", "LISTEN_FDS", "LISTEN_FDNAMES"} {
		os.Unsetenv(k)
	}
	if n != "1" {
		return nil, true, fmt.Errorf("socket activation requires exactly one socket, got LISTEN_FDS=%q", n)
	}
	f := os.NewFile(listenFDsStart, "systemd-socket")
	ln, e = net.FileListener(f)
	f.Close()
	if e != nil {
		return nil, true, fmt.Errorf("inherited socket: %w", e)
	}
	if a, ok := ln.Addr().(*net.TCPAddr); !ok || !a.IP.IsLoopback() {
		ln.Close()
		return nil, true, fmt.Errorf("inherited socket must be a loopback TCP address, got %s", ln.Addr())
	}
	return ln, true, nil
}
