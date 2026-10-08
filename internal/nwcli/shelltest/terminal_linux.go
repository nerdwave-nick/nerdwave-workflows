package shelltest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"unsafe"
)

// startOnTerminal starts cmd on a new pseudo-terminal and types input. It
// returns a function that stops cmd and one that returns its output so far.
func startOnTerminal(t testing.TB, cmd *exec.Cmd, input string) (stop func(), screen func() string) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal: %v", err)
	}
	ioctl := func(f *os.File, request uintptr, arg unsafe.Pointer) {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), request, uintptr(arg)); errno != 0 {
			t.Fatalf("ioctl %#x: %v", request, errno)
		}
	}
	unlock, n := int32(0), uint32(0)
	ioctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock))
	ioctl(master, syscall.TIOCGPTN, unsafe.Pointer(&n))
	size := [4]uint16{50, 200, 0, 0}
	ioctl(master, syscall.TIOCSWINSZ, unsafe.Pointer(&size))
	terminal, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = terminal, terminal, terminal
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	terminal.Close()
	var mu sync.Mutex
	var output bytes.Buffer
	go func() { // a shell blocks when nobody reads its output
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			mu.Lock()
			output.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	if _, err := io.WriteString(master, input); err != nil {
		t.Fatal(err)
	}
	stop = func() {
		cmd.Process.Kill()
		cmd.Wait()
		master.Close()
	}
	screen = func() string {
		mu.Lock()
		defer mu.Unlock()
		return output.String()
	}
	return stop, screen
}
