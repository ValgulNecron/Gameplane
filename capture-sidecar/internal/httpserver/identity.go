package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var captureUIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,128}$`)

var errCaptureIdentityBudget = errors.New("capture identity retention budget reached")

type captureIdentity struct {
	ServerUID  string `json:"serverUID"`
	CaptureUID string `json:"captureUID"`
}

func (s *Server) captureIdentityPath(id string) string {
	return filepath.Join(s.captureDataDir, "capture-"+id+".identity.json")
}

func (s *Server) validStartIdentity(req startRequest) bool {
	if req.ServerUID == "" && req.CaptureUID == "" {
		return true // Legacy local clients cannot use the bound file routes.
	}
	return captureUIDPattern.MatchString(req.ServerUID) && captureUIDPattern.MatchString(req.CaptureUID) && req.ServerUID == s.serverUID
}

// prepareCaptureBinding runs under s.mu before opening the writer. Linking a
// fully written temporary file publishes the identity atomically without ever
// replacing another capture's binding. A retained bound name cannot be reused,
// even by a legacy start, because the writer would truncate its PCAP.
func (s *Server) prepareCaptureBinding(id string, req startRequest) (bool, error) {
	if err := s.pruneCaptureTombstones(); err != nil {
		return false, err
	}
	path := s.captureIdentityPath(id)
	if _, err := os.Lstat(path); err == nil {
		return false, fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if req.CaptureUID == "" {
		return false, nil
	}
	if _, err := os.Lstat(s.captureFilePath(id)); err == nil {
		return false, fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	raw, err := json.Marshal(captureIdentity{ServerUID: req.ServerUID, CaptureUID: req.CaptureUID})
	if err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(s.captureDataDir, ".capture-identity-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Link(tmp.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}

// openCaptureRegular anchors access to the capture directory, rejecting symlinks
// and replacement between Lstat and Open. Callers supply a validated basename.
func (s *Server) openCaptureRegular(name string) (*os.File, error) {
	if name != filepath.Base(name) {
		return nil, fs.ErrInvalid
	}
	root, err := os.OpenRoot(s.captureDataDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fs.ErrInvalid
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		_ = file.Close()
		return nil, fs.ErrInvalid
	}
	return file, nil
}

func (s *Server) validCaptureTarget(r *http.Request) bool {
	id, serverUID, captureUID := r.PathValue("id"), r.PathValue("serverUID"), r.PathValue("captureUID")
	return validCaptureID(id) && captureUIDPattern.MatchString(serverUID) && captureUIDPattern.MatchString(captureUID) && serverUID == s.serverUID && r.URL.RawQuery == "" && r.URL.RawPath == ""
}

// matchCaptureIdentity reads durable metadata, not the bounded status cache.
// Old sidecars' unbound files intentionally have no remote download path.
func (s *Server) matchCaptureIdentity(r *http.Request) bool {
	if !s.validCaptureTarget(r) {
		return false
	}
	id, serverUID, captureUID := r.PathValue("id"), r.PathValue("serverUID"), r.PathValue("captureUID")
	file, err := s.openCaptureRegular("capture-" + id + ".identity.json")
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, 1025))
	if err != nil || len(raw) > 1024 {
		return false
	}
	var identity captureIdentity
	return json.Unmarshal(raw, &identity) == nil && identity.ServerUID == serverUID && identity.CaptureUID == captureUID
}

// captureDataAbsent runs under s.mu after target validation and the busy check.
// Lstat counts even dangling symlinks as present. An inaccessible directory or
// any error other than a missing entry cannot acknowledge cleanup.
func (s *Server) captureDataAbsent(id string) bool {
	root, err := os.OpenRoot(s.captureDataDir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	for _, suffix := range []string{".identity.json", ".pcapng"} {
		if _, err := root.Lstat("capture-" + id + suffix); !errors.Is(err, fs.ErrNotExist) {
			return false
		}
	}
	return true
}

// HandleBoundDownload opens the immutable pair while locked; a later deletion
// cannot switch an existing stream to a same-name replacement.
func (s *Server) HandleBoundDownload(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.matchCaptureIdentity(r) {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("id")
	if s.captureBusyLocked(id) {
		s.mu.Unlock()
		http.Error(w, "capture is still running", http.StatusConflict)
		return
	}
	file, err := s.openCaptureRegular("capture-" + id + ".pcapng")
	s.mu.Unlock()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "capture file has been deleted", http.StatusGone)
		} else {
			http.Error(w, "capture file unavailable", http.StatusInternalServerError)
		}
		return
	}
	defer func() { _ = file.Close() }()
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "capture file unavailable", http.StatusInternalServerError)
		return
	}
	name := fmt.Sprintf("capture-%s.pcapng", id)
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeContent(w, r, name, stat.ModTime(), file)
}

// HandleBoundDelete never removes a file whose persisted identities differ.
func (s *Server) HandleBoundDelete(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validCaptureTarget(r) {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("id")
	if s.captureBusyLocked(id) {
		http.Error(w, "capture is still running", http.StatusConflict)
		return
	}
	if s.captureDataAbsent(id) {
		http.Error(w, "capture data already absent", http.StatusGone)
		return
	}
	if !s.matchCaptureIdentity(r) {
		http.NotFound(w, r)
		return
	}
	if err := os.Remove(s.captureFilePath(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "capture file cleanup failed", http.StatusInternalServerError)
		return
	}
	// Keep this exact identity as a tombstone so retries after a lost response
	// or failed CR deletion still acknowledge cleanup without trusting a 404.
	now := time.Now()
	if err := os.Chtimes(s.captureIdentityPath(id), now, now); err != nil {
		http.Error(w, "capture cleanup acknowledgement unavailable", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hasControlIdentity(r *http.Request) bool {
	return len(r.Header.Values("X-Gameplane-Server-UID")) > 0 || len(r.Header.Values("X-Gameplane-Capture-UID")) > 0
}

func (s *Server) controlIdentityMatches(r *http.Request, state *captureState) bool {
	if !hasControlIdentity(r) {
		return state != nil
	}
	serverUID, captureUID := r.Header.Get("X-Gameplane-Server-UID"), r.Header.Get("X-Gameplane-Capture-UID")
	return state != nil && captureUIDPattern.MatchString(serverUID) && captureUIDPattern.MatchString(captureUID) && serverUID == s.serverUID && state.serverUID == serverUID && state.captureUID == captureUID
}

// finish publishes a completed-cache entry before flushing. Its state remains
// running until Close finishes, so it must continue to block file operations.
func (s *Server) captureBusyLocked(id string) bool {
	if s.currentCapture != nil && s.currentCapture.id == id {
		return true
	}
	if s.finishingCapture != nil && s.finishingCapture.id == id {
		return true
	}
	state := s.completed[id]
	return state != nil && state.snapshot().status == statusRunning
}

// Seven days is the CRD's maximum retention. Only file-less tombstones older
// than that window are reclaimed; live PCAP bindings remain immutable. Cap the
// tiny metadata files as well, so a long-lived pod cannot exhaust its inodes.
func (s *Server) pruneCaptureTombstones() error {
	paths, err := filepath.Glob(filepath.Join(s.captureDataDir, "capture-*.identity.json"))
	if err != nil {
		return err
	}
	remaining := 0
	for _, path := range paths {
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "capture-"), ".identity.json")
		if !validCaptureID(id) {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		_, fileErr := os.Lstat(s.captureFilePath(id))
		if info.Mode().IsRegular() && time.Since(info.ModTime()) > 7*24*time.Hour && errors.Is(fileErr, fs.ErrNotExist) {
			if err := os.Remove(path); err != nil {
				return err
			}
		} else {
			remaining++
		}
	}
	if remaining >= 4096 {
		return errCaptureIdentityBudget
	}
	return nil
}
