package gatewayprotocol

import (
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// CaptureTarget binds a sidecar file to both immutable Kubernetes identities.
type CaptureTarget struct {
	Target
	Capture    string
	CaptureUID string
}

// CapturePath is the sole private capture-file route; it never accepts a URL.
func CapturePath(target CaptureTarget) (string, error) {
	base, err := Path(target.Target, "/status")
	if err != nil || len(target.Capture) > 64 || len(validation.IsDNS1123Subdomain(target.Capture)) != 0 || !uidPattern.MatchString(target.CaptureUID) {
		return "", errors.New("invalid capture target")
	}
	return strings.TrimSuffix(base, "/status") + "/captures/" + target.Capture + "/uids/" + target.CaptureUID + "/file", nil
}

// ParseCapturePath accepts only the canonical immutable capture-file route.
func ParseCapturePath(path string) (CaptureTarget, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 15 || parts[0] != "" || parts[1] != "v1" || parts[2] != "clusters" || parts[4] != "namespaces" || parts[6] != "servers" || parts[8] != "uids" || parts[10] != "captures" || parts[12] != "uids" || parts[14] != "file" {
		return CaptureTarget{}, errors.New("invalid capture route")
	}
	target := CaptureTarget{Target: Target{Cluster: parts[3], Namespace: parts[5], Name: parts[7], UID: parts[9]}, Capture: parts[11], CaptureUID: parts[13]}
	canonical, err := CapturePath(target)
	if err != nil || canonical != path {
		return CaptureTarget{}, errors.New("invalid capture route")
	}
	return target, nil
}
