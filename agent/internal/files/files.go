// Package files serves the file-browser HTTP API. All paths are
// resolved relative to a fixed root (e.g. /data) and path-traversal
// attempts (.., symlinks escaping the root) are rejected.
package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// errPathOutOfRoot is the only "bad path" error safe to echo back to the
// client. Anything else (EvalSymlinks filesystem errors, etc.) might leak
// absolute paths or volume internals — those get logged and 400'd with
// a generic message. Handlers route through badRequest() so the
// classification is enforced in one place.
var errPathOutOfRoot = errors.New("path escapes root")

type handler struct {
	root string
}

func Mount(r chi.Router, root string) {
	h := &handler{root: filepath.Clean(root)}
	r.Route("/files", func(r chi.Router) {
		r.Get("/list", h.list)
		r.Get("/read", h.read)
		r.Get("/download", h.download)
		r.Post("/write", h.write)
		r.Post("/upload", h.upload)
		r.Post("/mkdir", h.mkdir)
		r.Delete("/delete", h.del)
	})
}

// Entry is one directory item returned by /files/list.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	Dir     bool   `json:"dir"`
	ModTime string `json:"modTime"`
}

func (h *handler) resolve(rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return h.root, nil
	}
	abs := filepath.Join(h.root, filepath.Clean("/"+rel))
	if !strings.HasPrefix(abs, h.root+string(os.PathSeparator)) && abs != h.root {
		return "", errPathOutOfRoot
	}
	// Evaluate symlinks to block the "/data/escape -> /etc/passwd"
	// attack: a prefix check on the raw path passes, but the real read
	// hits the linked target. If the path doesn't exist yet (a new
	// file about to be written), EvalSymlinks returns an error — fall
	// back to checking the deepest existing ancestor.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if !strings.HasPrefix(resolved, h.root+string(os.PathSeparator)) && resolved != h.root {
			return "", errPathOutOfRoot
		}
		return resolved, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	// The target doesn't exist yet (a new file/dir about to be written).
	// Walk up to the deepest ancestor that *does* exist and verify it
	// resolves inside the root, so the symlink-escape check still holds
	// while allowing nested creation (mkdir -p, or uploading into a new
	// subtree). Checking only the immediate parent rejected "/one/two/file"
	// whenever "/one" was absent, making the later MkdirAll unreachable. The
	// loop terminates at worst at the filesystem root, which always exists.
	for parent := filepath.Dir(abs); ; parent = filepath.Dir(parent) {
		resolvedParent, err := filepath.EvalSymlinks(parent)
		if err == nil {
			if !strings.HasPrefix(resolvedParent, h.root+string(os.PathSeparator)) && resolvedParent != h.root {
				return "", errPathOutOfRoot
			}
			return abs, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
}

// badRequest writes a 400 with a client-safe message. errPathOutOfRoot is
// the one class we echo verbatim — everything else (EvalSymlinks errors,
// multipart parse details, etc.) is logged and replaced with a generic
// "bad request" so filesystem/implementation details stay inside the pod.
func (h *handler) badRequest(w http.ResponseWriter, err error) {
	if errors.Is(err, errPathOutOfRoot) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slog.Warn("files bad request", "err", err)
	http.Error(w, "bad request", http.StatusBadRequest)
}

func (h *handler) list(w http.ResponseWriter, req *http.Request) {
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		httpErr(w, err)
		return
	}
	out := make([]Entry, 0, len(ents))
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(h.root, filepath.Join(p, e.Name()))
		out = append(out, Entry{
			Name:    e.Name(),
			Path:    "/" + filepath.ToSlash(rel),
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			Dir:     fi.IsDir(),
			ModTime: fi.ModTime().UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, out)
}

func (h *handler) read(w http.ResponseWriter, req *http.Request) {
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	fi, err := os.Stat(p)
	if err != nil {
		httpErr(w, err)
		return
	}
	if fi.IsDir() {
		http.Error(w, "is a directory", http.StatusBadRequest)
		return
	}
	const maxRead = 2 << 20 // 2 MiB inline read
	if fi.Size() > maxRead {
		http.Error(w, "file too large; use /files/download", http.StatusRequestEntityTooLarge)
		return
	}
	http.ServeFile(w, req, p)
}

func (h *handler) download(w http.ResponseWriter, req *http.Request) {
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	fi, err := os.Stat(p)
	if err != nil {
		httpErr(w, err)
		return
	}
	if fi.IsDir() {
		http.Error(w, "download of directories not supported", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename=%q`, filepath.Base(p)))
	http.ServeFile(w, req, p)
}

// maxWriteBytes caps a single /files/write body. The API-side ws proxy
// already enforces 64 MiB, so this is defense-in-depth against direct
// agent access with a valid mTLS cert.
const maxWriteBytes = 64 << 20

// maxUploadFileBytes caps each individual file in a multipart upload.
// ParseMultipartForm's 64 MiB limit is a *total* budget — without a
// per-file cap, one oversized file is still allowed as long as the
// multipart metadata fits.
const maxUploadFileBytes = 64 << 20

// maxUploadFiles caps the number of attachments per request. Reasonable
// uploads (config patches, mod files) come in small counts; thousands
// of tiny files are an attempted inode-exhaustion DoS.
const maxUploadFiles = 64

func (h *handler) write(w http.ResponseWriter, req *http.Request) {
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	defer req.Body.Close()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		httpErr(w, err)
		return
	}
	f, err := os.Create(p)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer f.Close()
	if _, err := io.Copy(f, http.MaxBytesReader(w, req.Body, maxWriteBytes)); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) upload(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(64 << 20); err != nil {
		h.badRequest(w, err)
		return
	}
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		httpErr(w, err)
		return
	}
	count := 0
	for _, fhs := range req.MultipartForm.File {
		count += len(fhs)
	}
	if count > maxUploadFiles {
		h.badRequest(w, fmt.Errorf("too many files (max %d)", maxUploadFiles))
		return
	}
	for _, fhs := range req.MultipartForm.File {
		for _, fh := range fhs {
			if err := saveMultipart(p, fh); err != nil {
				httpErr(w, err)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func saveMultipart(dir string, fh *multipart.FileHeader) error {
	if fh.Size > maxUploadFileBytes {
		return fmt.Errorf("file %q exceeds %d-byte limit", filepath.Base(fh.Filename), maxUploadFileBytes)
	}
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	// Sanitize filename — reject anything that would climb out of dir.
	name := filepath.Base(fh.Filename)
	if name == "." || name == ".." || name == string(os.PathSeparator) {
		return errors.New("invalid filename")
	}
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer dst.Close()
	// Even with Size checked above, cap the copy so a lying multipart
	// header can't bypass it.
	_, err = io.Copy(dst, io.LimitReader(src, maxUploadFileBytes))
	return err
}

func (h *handler) mkdir(w http.ResponseWriter, req *http.Request) {
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) del(w http.ResponseWriter, req *http.Request) {
	p, err := h.resolve(req.URL.Query().Get("path"))
	if err != nil {
		h.badRequest(w, err)
		return
	}
	if p == h.root {
		http.Error(w, "refusing to delete root", http.StatusBadRequest)
		return
	}
	recursive := req.URL.Query().Get("recursive") == "true"
	var rerr error
	if recursive {
		rerr = os.RemoveAll(p)
	} else {
		rerr = os.Remove(p)
	}
	if rerr != nil {
		httpErr(w, rerr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func httpErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, os.ErrPermission):
		http.Error(w, "forbidden", http.StatusForbidden)
	default:
		// Never echo the raw error — it frequently contains absolute
		// paths, syscall details, or PVC mount info.
		slog.Warn("files internal error", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
