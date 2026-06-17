// Package usage reads the agent's own resource consumption from inside
// the game pod and exposes it as a Sample for the heartbeat to report.
//
// Everything is sourced in-pod with no cluster-level metrics pipeline
// (Kestrel deliberately has no metrics-server dependency): CPU and memory
// come from the pod's cgroup v2 files, disk from a statfs of the game
// data volume. Each value carries a "known" flag so callers can patch
// null ("unknown") rather than a misleading zero when a source is
// unreadable — which is normal in dev environments and on cgroup v1.
package usage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// defaultCgroupRoot is the cgroup v2 unified-hierarchy mount present in
// every modern pod (Kubernetes 1.28+ defaults to cgroup v2).
const defaultCgroupRoot = "/sys/fs/cgroup"

// Config selects where the reader looks. Root and DataDir are the public
// knobs; now and statfs are injected only by tests so they can feed
// fixtures and a deterministic clock instead of touching the real host.
type Config struct {
	// Root is the cgroup v2 mount point. Empty means defaultCgroupRoot.
	Root string
	// DataDir is the game data volume to statfs for disk usage. Empty
	// disables the disk reading.
	DataDir string

	now    func() time.Time
	statfs func(string, *unix.Statfs_t) error
}

// Sample is one point-in-time reading. Each *Known flag reports whether
// the matching value could be sourced; an unknown value must be reported
// as null, never zero, so the dashboard can render "—".
type Sample struct {
	CPUMillicores      int64
	CPUKnown           bool
	CPULimitMillicores int64
	CPULimitKnown      bool

	MemoryBytes      int64
	MemoryKnown      bool
	MemoryLimitBytes int64
	MemoryLimitKnown bool

	DiskUsedBytes  int64
	DiskTotalBytes int64
	DiskKnown      bool
}

// Reader produces Samples. It holds the previous CPU usage counter so it
// can turn cgroup's cumulative usage_usec into a rate; keeping that state
// here (not in the heartbeat loop) lets the heartbeat's sendOnce stay a
// pure function of its Config. Read is safe for concurrent use.
type Reader struct {
	cfg Config

	mu            sync.Mutex
	prevUsageUsec uint64
	prevAt        time.Time
	havePrev      bool
}

// New builds a Reader, applying defaults for any unset Config field.
func New(cfg Config) *Reader {
	if cfg.Root == "" {
		cfg.Root = defaultCgroupRoot
	}
	cfg.Root = filepath.Clean(cfg.Root)
	if cfg.now == nil {
		cfg.now = time.Now
	}
	if cfg.statfs == nil {
		cfg.statfs = unix.Statfs
	}
	return &Reader{cfg: cfg}
}

// Read takes one sample. It never returns an error: any unreadable source
// leaves its value zero with the corresponding Known flag false. The first
// call after construction reports an unknown CPU rate (no prior counter to
// diff against) — the next call, one heartbeat later, has it.
func (r *Reader) Read() Sample {
	var s Sample
	r.readCPU(&s)
	r.readMemory(&s)
	r.readDisk(&s)
	return s
}

func (r *Reader) readCPU(s *Sample) {
	if usageUsec, ok := r.cpuUsageUsec(); ok {
		r.mu.Lock()
		now := r.cfg.now()
		if r.havePrev {
			dWall := now.Sub(r.prevAt).Microseconds()
			// Guard against a clock that didn't advance and a counter
			// that went backwards (shouldn't happen, but a reset would
			// otherwise yield a wild rate).
			if dWall > 0 && usageUsec >= r.prevUsageUsec {
				dUsage := int64(usageUsec - r.prevUsageUsec)
				s.CPUMillicores = dUsage * 1000 / dWall
				s.CPUKnown = true
			}
		}
		r.prevUsageUsec = usageUsec
		r.prevAt = now
		r.havePrev = true
		r.mu.Unlock()
	}

	if lim, ok := r.cpuLimitMillicores(); ok {
		s.CPULimitMillicores = lim
		s.CPULimitKnown = true
	}
}

// cpuUsageUsec returns the cumulative CPU time in microseconds from the
// first "usage_usec" line of cgroup v2's cpu.stat.
func (r *Reader) cpuUsageUsec() (uint64, bool) {
	b, err := os.ReadFile(filepath.Join(r.cfg.Root, "cpu.stat"))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "usage_usec" {
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}

// cpuLimitMillicores parses cgroup v2's cpu.max ("quota period"). A quota
// of "max" means no limit, which we report as unknown.
func (r *Reader) cpuLimitMillicores() (int64, bool) {
	b, err := os.ReadFile(filepath.Join(r.cfg.Root, "cpu.max"))
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(b))
	if len(fields) != 2 || fields[0] == "max" {
		return 0, false
	}
	quota, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, false
	}
	period, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || period <= 0 {
		return 0, false
	}
	return quota * 1000 / period, true
}

func (r *Reader) readMemory(s *Sample) {
	if v, ok := readUintFile(filepath.Join(r.cfg.Root, "memory.current")); ok {
		s.MemoryBytes = int64(v)
		s.MemoryKnown = true
	}
	// memory.max is "max" when unlimited, which fails to parse — exactly
	// the "unknown limit" we want to report.
	if v, ok := readUintFile(filepath.Join(r.cfg.Root, "memory.max")); ok {
		s.MemoryLimitBytes = int64(v)
		s.MemoryLimitKnown = true
	}
}

func (r *Reader) readDisk(s *Sample) {
	if r.cfg.DataDir == "" {
		return
	}
	var st unix.Statfs_t
	if err := r.cfg.statfs(r.cfg.DataDir, &st); err != nil {
		return
	}
	// Bsize's Go type varies by arch (int64 on amd64/arm64); convert via
	// uint64 so the arithmetic is correct everywhere and no conversion is
	// redundant.
	bsize := uint64(st.Bsize)
	if bsize == 0 {
		return
	}
	s.DiskTotalBytes = int64(st.Blocks * bsize)
	s.DiskUsedBytes = int64((st.Blocks - st.Bfree) * bsize)
	s.DiskKnown = true
}

// readUintFile parses a cgroup file holding a single unsigned integer.
func readUintFile(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
