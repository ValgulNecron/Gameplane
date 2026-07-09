//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// probeNamespace is where the game-bot probe Job runs. It must NOT be the games
// namespace: the chart installs a default-deny-egress NetworkPolicy there with
// `podSelector: {}`, so every pod in it — including the probe — may egress only
// to DNS, and the probe's connect to the game port would be dropped. From
// outside that namespace the probe's egress is unrestricted, and the game pod's
// own `allow-kubelet-probes` policy admits ingress from any RFC1918 pod IP.
// This also mirrors how a real player reaches the game: from off-cluster.
const probeNamespace = "default"

// probeImage is the in-cluster game-bot image: built by the game-bot CI job
// (docker-bake.hcl target "e2e-gameprobe") and side-loaded into kind by
// deploy/kind/e2e.sh. Override it when the cluster pulls from a registry
// instead (e.g. a reused remote cluster).
func (e *Env) probeImage() string {
	if img := os.Getenv("GAMEPLANE_E2E_PROBE_IMAGE"); img != "" {
		return img
	}
	tag := e.Tag
	if tag == "" {
		tag = "e2e"
	}
	return "gameplane-test/gameprobe:" + tag
}

// RunGameProbe runs the headless protocol bot as a Job inside the cluster,
// pointed at the game Service's DNS name, and fails the test unless it exits 0.
//
// The bot runs in-cluster on purpose. Reaching the game through
// `kubectl port-forward` carries the game protocol over an apiserver SPDY
// tunnel which, under CI load, corrupts the Minecraft login handshake — the
// server then drops the connection ("Failed to decode packet
// 'serverbound/minecraft:hello'"). Dialing the Service directly removes that hop.
//
// gameprobe owns its own retry loop (a game server accepts TCP well before it
// accepts a login), so this waits for a single terminal Job outcome rather than
// retrying the protocol here.
//
// The Job runs in probeNamespace (default) and dials the game Service in gameNS.
func (e *Env) RunGameProbe(t *testing.T, gameNS, gsName, game string, port int, deadline time.Duration) {
	t.Helper()
	ctx := context.Background()
	jobName := gsName + "-probe"
	addr := fmt.Sprintf("%s.%s.svc.cluster.local:%d", gsName, gameNS, port)

	bg := metav1.DeletePropagationBackground
	// Recreate so a previous failure can't leave a Failed shell behind.
	_ = e.K8s.BatchV1().Jobs(probeNamespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &bg})
	t.Cleanup(func() {
		_ = e.K8s.BatchV1().Jobs(probeNamespace).Delete(
			context.Background(), jobName, metav1.DeleteOptions{PropagationPolicy: &bg})
	})

	var (
		nonRoot   = true
		noPrivEsc = false
		roRootFS  = true
		uid       = int64(65532)
		backoff   = int32(0)
	)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: probeNamespace},
		Spec: batchv1.JobSpec{
			// One shot: gameprobe already retries internally, so a non-zero
			// exit means the server never became playable.
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   &nonRoot,
						RunAsUser:      &uid,
						RunAsGroup:     &uid,
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					Containers: []corev1.Container{{
						Name:  "gameprobe",
						Image: e.probeImage(),
						// Side-loaded into kind; never pulled from a registry.
						ImagePullPolicy: corev1.PullNever,
						Args: []string{
							"-game", game,
							"-addr", addr,
							"-deadline", deadline.String(),
						},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &nonRoot,
							AllowPrivilegeEscalation: &noPrivEsc,
							ReadOnlyRootFilesystem:   &roRootFS,
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
				},
			},
		},
	}
	if _, err := e.K8s.BatchV1().Jobs(probeNamespace).Create(ctx, job, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create %s probe job: %v", game, err)
	}

	// Allow a little more than the probe's own deadline, so a probe timeout
	// surfaces as its logged reason rather than as this wait expiring.
	wait := deadline + 2*time.Minute
	expiry := time.Now().Add(wait)
	for {
		j, err := e.K8s.BatchV1().Jobs(probeNamespace).Get(ctx, jobName, metav1.GetOptions{})
		if err == nil {
			if j.Status.Succeeded > 0 {
				out, _ := e.Kubectl("logs", "-n", probeNamespace, "job/"+jobName, "--tail=50")
				t.Logf("%s probe passed against %s:\n%s", game, addr, out)
				return
			}
			if j.Status.Failed > 0 {
				out, _ := e.Kubectl("logs", "-n", probeNamespace, "job/"+jobName, "--tail=200")
				t.Fatalf("%s probe failed — %s never became playable:\n%s", game, addr, out)
			}
		}
		if time.Now().After(expiry) {
			out, _ := e.Kubectl("logs", "-n", probeNamespace, "job/"+jobName, "--tail=200")
			pods, _ := e.Kubectl("get", "pods", "-n", probeNamespace, "-l", "job-name="+jobName, "-o", "wide")
			t.Fatalf("%s probe job did not finish within %s (is %s loaded into the cluster?)\npods:\n%s\nlogs:\n%s",
				game, wait, e.probeImage(), pods, out)
		}
		time.Sleep(5 * time.Second)
	}
}
