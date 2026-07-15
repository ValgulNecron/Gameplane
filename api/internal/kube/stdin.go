package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// stdinAttachTimeout bounds one WriteStdinLines call. Pod attach sessions
// are tied to the container's lifetime, not to stdin going EOF, so the
// connection doesn't close itself once the write completes — this client
// tears it down deliberately once the write is done. The timeout is a
// backstop for a slow/hung negotiation; it is not how the normal case ends.
const stdinAttachTimeout = 10 * time.Second

// WriteStdinLines attaches to a game container's stdin via pods/attach and
// writes each line followed by \n, then tears the session down. Fire-and-
// forget: pod attach doesn't EOF-close, so it's bounded by a timeout. Mirrors
// the operator's stop-attach (gameserver_stop_attach.go) and the Console
// tab's attach shape (api/internal/ws/attach.go) — TTY:true, Stdout
// requested + discarded — a tty/non-tty mismatch behaves differently across
// container runtimes.
func (c *Client) WriteStdinLines(ctx context.Context, ns, pod, container string, lines []string) error {
	url := c.Typed.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(ns).
		SubResource("attach").
		VersionedParams(&corev1.PodAttachOptions{
			Container: container,
			Stdin:     true,
			Stdout:    true,
			Stderr:    false, // merged with stdout under TTY
			TTY:       true,
		}, scheme.ParameterCodec).
		URL()

	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", url)
	if err != nil {
		return fmt.Errorf("build stdin attach executor for pod %s/%s: %w", ns, pod, err)
	}

	var stdin bytes.Buffer
	for _, line := range lines {
		stdin.WriteString(line)
		stdin.WriteString("\n")
	}

	// Bound the call ourselves rather than blocking on the remote side
	// ending the session: attach doesn't end just because stdin EOFs, so
	// without this the call would hang until the game process exits.
	streamCtx, cancel := context.WithTimeout(ctx, stdinAttachTimeout)
	defer cancel()

	err = exec.StreamWithContext(streamCtx, remotecommand.StreamOptions{
		Stdin:  &stdin,
		Stdout: io.Discard,
		Tty:    true,
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("write stdin to pod %s/%s: %w", ns, pod, err)
	}
	return nil
}
