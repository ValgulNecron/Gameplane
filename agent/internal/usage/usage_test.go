package usage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

// TestRead_CPURate verifies the cumulative usage_usec counter is turned
// into a millicores rate using the wall-clock delta, and that the first
// read (no prior sample) reports an unknown rate while still reading the
// limit.
func TestRead_CPURate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.stat", "usage_usec 1000000\nuser_usec 1\nsystem_usec 1\n")
	writeFile(t, dir, "cpu.max", "200000 100000\n")

	clock := &fakeClock{t: time.Unix(100, 0)}
	r := New(Config{Root: dir, now: clock.now})

	s1 := r.Read()
	if s1.CPUKnown {
		t.Fatalf("first read should report unknown CPU rate, got %d", s1.CPUMillicores)
	}
	if !s1.CPULimitKnown || s1.CPULimitMillicores != 2000 {
		t.Fatalf("cpu limit = %d known=%v, want 2000", s1.CPULimitMillicores, s1.CPULimitKnown)
	}

	// 2 CPU-seconds consumed over 2 wall-seconds = exactly 1 core.
	clock.t = clock.t.Add(2 * time.Second)
	writeFile(t, dir, "cpu.stat", "usage_usec 3000000\n")
	s2 := r.Read()
	if !s2.CPUKnown || s2.CPUMillicores != 1000 {
		t.Fatalf("cpu millicores = %d known=%v, want 1000", s2.CPUMillicores, s2.CPUKnown)
	}
}

// TestRead_CPUNoWallAdvance covers the guard where the clock did not move
// between samples — no rate can be computed.
func TestRead_CPUNoWallAdvance(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.stat", "usage_usec 5\n")
	clock := &fakeClock{t: time.Unix(100, 0)}
	r := New(Config{Root: dir, now: clock.now})

	r.Read() // seed prev
	writeFile(t, dir, "cpu.stat", "usage_usec 10\n")
	s := r.Read() // same clock value
	if s.CPUKnown {
		t.Fatalf("no wall advance should leave CPU unknown, got %d", s.CPUMillicores)
	}
}

// TestRead_CPUCounterReset covers the guard where the cumulative counter
// goes backwards (a cgroup/container reset) — no rate is emitted rather
// than a wild negative-turned-huge value.
func TestRead_CPUCounterReset(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.stat", "usage_usec 5000000\n")
	clock := &fakeClock{t: time.Unix(100, 0)}
	r := New(Config{Root: dir, now: clock.now})

	r.Read() // seed prev at 5,000,000
	clock.t = clock.t.Add(2 * time.Second)
	writeFile(t, dir, "cpu.stat", "usage_usec 1000000\n") // counter went backwards
	s := r.Read()
	if s.CPUKnown {
		t.Fatalf("a backwards usage counter should leave CPU unknown, got %d", s.CPUMillicores)
	}
}

func TestRead_CPUMaxUnlimited(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.max", "max 100000\n")
	s := New(Config{Root: dir}).Read()
	if s.CPULimitKnown {
		t.Fatalf("unlimited cpu.max should report unknown limit")
	}
}

func TestRead_Memory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "memory.current", "536870912\n")
	writeFile(t, dir, "memory.max", "1073741824\n")
	s := New(Config{Root: dir}).Read()
	if !s.MemoryKnown || s.MemoryBytes != 536870912 {
		t.Fatalf("memory = %d known=%v", s.MemoryBytes, s.MemoryKnown)
	}
	if !s.MemoryLimitKnown || s.MemoryLimitBytes != 1073741824 {
		t.Fatalf("memory limit = %d known=%v", s.MemoryLimitBytes, s.MemoryLimitKnown)
	}
}

func TestRead_MemoryMaxUnlimited(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "memory.current", "1024\n")
	writeFile(t, dir, "memory.max", "max\n")
	s := New(Config{Root: dir}).Read()
	if !s.MemoryKnown || s.MemoryBytes != 1024 {
		t.Fatalf("memory = %d known=%v", s.MemoryBytes, s.MemoryKnown)
	}
	if s.MemoryLimitKnown {
		t.Fatalf("unlimited memory.max should report unknown limit")
	}
}

func TestRead_Disk(t *testing.T) {
	r := New(Config{
		Root:    t.TempDir(),
		DataDir: "/data",
		statfs: func(path string, st *unix.Statfs_t) error {
			if path != "/data" {
				t.Fatalf("statfs path = %q", path)
			}
			st.Bsize = 4096
			st.Blocks = 1000
			st.Bfree = 250
			return nil
		},
	})
	s := r.Read()
	if !s.DiskKnown {
		t.Fatalf("disk should be known")
	}
	if s.DiskTotalBytes != 4096*1000 {
		t.Fatalf("disk total = %d", s.DiskTotalBytes)
	}
	if s.DiskUsedBytes != 4096*750 {
		t.Fatalf("disk used = %d", s.DiskUsedBytes)
	}
}

func TestRead_DiskDisabledWhenNoDataDir(t *testing.T) {
	s := New(Config{Root: t.TempDir()}).Read()
	if s.DiskKnown {
		t.Fatalf("disk should be disabled without a DataDir")
	}
}

func TestRead_DiskStatfsError(t *testing.T) {
	r := New(Config{
		Root:    t.TempDir(),
		DataDir: "/data",
		statfs:  func(string, *unix.Statfs_t) error { return errors.New("boom") },
	})
	if r.Read().DiskKnown {
		t.Fatalf("disk should be unknown on statfs error")
	}
}

func TestRead_DiskZeroBlockSize(t *testing.T) {
	r := New(Config{
		Root:    t.TempDir(),
		DataDir: "/data",
		statfs: func(_ string, st *unix.Statfs_t) error {
			st.Bsize = 0
			return nil
		},
	})
	if r.Read().DiskKnown {
		t.Fatalf("disk should be unknown when block size is zero")
	}
}

func TestRead_AllUnknownWhenAbsent(t *testing.T) {
	s := New(Config{Root: t.TempDir()}).Read()
	if s.CPUKnown || s.CPULimitKnown || s.MemoryKnown || s.MemoryLimitKnown || s.DiskKnown {
		t.Fatalf("everything should be unknown on an empty cgroup root: %+v", s)
	}
}

func TestRead_MalformedValues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.stat", "user_usec 1\nusage_usec notanumber\n")
	writeFile(t, dir, "cpu.max", "abc def\n")
	writeFile(t, dir, "memory.current", "xyz\n")
	s := New(Config{Root: dir}).Read()
	if s.CPUKnown || s.CPULimitKnown || s.MemoryKnown {
		t.Fatalf("malformed values should be unknown: %+v", s)
	}
}

func TestRead_CPUStatWithoutUsageLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.stat", "user_usec 5\nsystem_usec 3\n")
	s := New(Config{Root: dir}).Read()
	if s.CPUKnown {
		t.Fatalf("cpu.stat without usage_usec should be unknown")
	}
}

func TestRead_CPUMaxWrongFieldCount(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.max", "100000\n")
	s := New(Config{Root: dir}).Read()
	if s.CPULimitKnown {
		t.Fatalf("malformed cpu.max should be unknown")
	}
}

func TestRead_CPUMaxZeroPeriod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cpu.max", "100000 0\n")
	s := New(Config{Root: dir}).Read()
	if s.CPULimitKnown {
		t.Fatalf("zero period should be unknown")
	}
}

func TestNew_DefaultsRoot(t *testing.T) {
	r := New(Config{})
	if r.cfg.Root != defaultCgroupRoot {
		t.Fatalf("default root = %q, want %q", r.cfg.Root, defaultCgroupRoot)
	}
	if r.cfg.now == nil || r.cfg.statfs == nil {
		t.Fatalf("now/statfs should default to non-nil")
	}
}
