package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// attachStopTimeout bounds one PodStopAttacher.Stop call. Kubernetes pod
// attach sessions are tied to the container's lifetime, not to stdin going
// EOF (unlike an exec of a short-lived process), so the connection doesn't
// close itself once the commands have been written — this client tears it
// down deliberately once the write completes. The timeout is a backstop
// for a slow/hung negotiation; it is not how the normal case ends.
const attachStopTimeout = 10 * time.Second

// PodStopAttacher writes the module-declared stop sequence to a game
// container's stdin via a Kubernetes pod attach, for consoleMode: pty
// games that declare no RCON (e.g. Terraria, Factorio without RCON).
// Satisfied by *StopAttachClient, built from the manager's rest.Config in
// cmd/main.go. May be nil — dev clusters or tests that don't wire it get
// the same fallback softStop applies to "no usable transport": scale
// straight to zero instead of waiting out the grace period.
type PodStopAttacher interface {
	Stop(ctx context.Context, namespace, podName, container string, commands []string) error
}

// StopAttachClient implements PodStopAttacher via the pods/attach
// subresource, mirroring the dashboard's own console-pty bridge
// (api/internal/ws/attach.go) but writing a canned command sequence
// instead of proxying an interactive terminal, and without a TTY — plain
// line-based stdin is enough to drive the game's stdin-reading command
// loop.
type StopAttachClient struct {
	Config    *rest.Config
	Clientset kubernetes.Interface
}

// Stop attaches to container's stdin in the named pod, writes each
// command followed by a newline, then tears the attach session down. It
// is best-effort: the caller (softStop) treats pod readiness and its own
// grace deadline as the authority on whether the game actually went
// down, so a failure here is only logged upstream, never surfaced as a
// reconcile error.
func (a *StopAttachClient) Stop(ctx context.Context, namespace, podName, container string, commands []string) error {
	url := a.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("attach").
		VersionedParams(&corev1.PodAttachOptions{
			Container: container,
			Stdin:     true,
			Stdout:    false,
			Stderr:    false,
			TTY:       false,
		}, clientgoscheme.ParameterCodec).
		URL()

	exec, err := remotecommand.NewSPDYExecutor(a.Config, "POST", url)
	if err != nil {
		return fmt.Errorf("build stop-sequence attach executor for pod %s/%s: %w", namespace, podName, err)
	}

	var stdin bytes.Buffer
	for _, cmd := range commands {
		stdin.WriteString(cmd)
		stdin.WriteString("\n")
	}

	// Bound the call ourselves rather than blocking on the remote side
	// ending the session: attach doesn't end just because stdin EOFs, so
	// without this the call would hang until the game process exits.
	streamCtx, cancel := context.WithTimeout(ctx, attachStopTimeout)
	defer cancel()

	err = exec.StreamWithContext(streamCtx, remotecommand.StreamOptions{Stdin: &stdin})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("attach stop sequence to pod %s/%s: %w", namespace, podName, err)
	}
	return nil
}
