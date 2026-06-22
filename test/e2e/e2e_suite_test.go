//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// envInstance is built once in TestMain and reused by every test.
var envInstance *Env

// TestMain optionally creates a kind cluster, runs all e2e tests
// against it, and tears it down at exit. Honors the env vars:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1
//	    Skip create + destroy. Use this for fast dev iteration once
//	    you've already run `deploy/kind/e2e.sh up`.
//
//	GAMEPLANE_E2E_CLUSTER=<name>   default: kestrel-e2e
//	GAMEPLANE_E2E_TAG=<tag>        default: e2e
//	GAMEPLANE_E2E_KEEP_ON_FAILURE=1
//	    Don't tear down if any test fails (helpful for post-mortem
//	    `kubectl get all -A` and operator log inspection).
func TestMain(m *testing.M) {
	reuse := os.Getenv("GAMEPLANE_E2E_REUSE_CLUSTER") == "1"

	if !reuse {
		if err := runBootstrap("up"); err != nil {
			fmt.Fprintf(os.Stderr, "bootstrap up failed: %v\n", err)
			os.Exit(1)
		}
	}

	env, err := newEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build env: %v\n", err)
		if !reuse {
			_ = runBootstrap("down")
		}
		os.Exit(1)
	}
	if err := env.ensureCluster(); err != nil {
		fmt.Fprintf(os.Stderr, "cluster not reachable: %v\n", err)
		if !reuse {
			_ = runBootstrap("down")
		}
		os.Exit(1)
	}
	envInstance = env

	code := m.Run()

	if !reuse {
		// On failure, optionally leave the cluster around for forensics.
		if code != 0 && os.Getenv("GAMEPLANE_E2E_KEEP_ON_FAILURE") == "1" {
			fmt.Fprintln(os.Stderr, "tests failed; leaving cluster up for inspection")
		} else {
			_ = runBootstrap("down")
		}
	}
	os.Exit(code)
}

// runBootstrap invokes deploy/kind/e2e.sh.
func runBootstrap(action string) error {
	script, err := findRepoRel("deploy/kind/e2e.sh")
	if err != nil {
		return err
	}
	cmd := exec.Command(script, action)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// findRepoRel walks up from cwd to find the repository root
// (identified by go.work) and returns repo+rel.
func findRepoRel(rel string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.work")); err == nil {
			return filepath.Join(cwd, rel), nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("could not find repo root from %s", cwd)
		}
		cwd = parent
	}
}
