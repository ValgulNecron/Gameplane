package logs

import (
	"net/http"
	"testing"
)

// A configured log path that points at a directory is a misconfiguration:
// the download handler must 500 rather than try to serve a directory.
func TestDownload_PathIsDirectory(t *testing.T) {
	url := mountServer(t, t.TempDir()) // the path is a directory
	resp, err := testGet(t, url+"/logs/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", resp.StatusCode)
	}
}
