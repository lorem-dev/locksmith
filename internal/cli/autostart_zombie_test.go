//go:build !windows

package cli_test

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestAutostart_ZombieReaping verifies that the goroutine used in autostart_cmd.go
// to call c.Wait() prevents short-lived child processes from becoming zombies.
// It mirrors the production pattern directly and checks ps(1) for Z-state children.
func TestAutostart_ZombieReaping(t *testing.T) {
	// Spawn a no-op process that exits immediately, simulating a daemon that
	// fails to start (e.g. due to a missing config file).
	c := exec.Command("true")
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	// Reap in a goroutine - this is the exact pattern in autostart_cmd.go.
	go func() { _ = c.Wait() }()

	// Poll ps for up to 1s and confirm no Z-state children of this process.
	myPID := os.Getpid()
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.Command("ps", "-eo", "pid,ppid,stat").Output()
		if err != nil {
			t.Fatalf("ps error: %v", err)
		}
		if zombies := zombieChildrenOf(string(out), myPID); len(zombies) == 0 {
			return // no zombies found - pass
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Final definitive check after 1s.
	out, _ := exec.Command("ps", "-eo", "pid,ppid,stat").Output()
	if zombies := zombieChildrenOf(string(out), myPID); len(zombies) > 0 {
		t.Errorf("zombie children of pid %d still present after 1s: PIDs %v", myPID, zombies)
	}
}

// zombieChildrenOf returns the PIDs of processes whose ppid equals parentPID and
// whose stat field starts with "Z" (zombie state).
func zombieChildrenOf(psOut string, parentPID int) []int {
	var zombies []int
	for _, line := range strings.Split(psOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil || ppid != parentPID {
			continue
		}
		if strings.HasPrefix(fields[2], "Z") {
			pid, _ := strconv.Atoi(fields[0])
			zombies = append(zombies, pid)
		}
	}
	return zombies
}
