package common

import (
	"testing"
)

func TestYAMLNodeLineFinder(t *testing.T) {
	yamlContent := `
apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
spec:
  displayName: Test Game
  image: example/image:latest
  ports:
    - name: game
      containerPort: 25565
      protocol: TCP
    - name: rcon
      containerPort: 25575
      protocol: TCP
`

	root, err := ParseYAMLNode([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseYAMLNode failed: %v", err)
	}

	lineImage := FindLineNumber(root, "spec.image")
	if lineImage != 6 {
		t.Errorf("expected line for spec.image to be 6, got %d", lineImage)
	}

	linePort0 := FindLineNumber(root, "spec.ports[0].containerPort")
	if linePort0 != 9 {
		t.Errorf("expected line for spec.ports[0].containerPort to be 9, got %d", linePort0)
	}

	linePort1 := FindLineNumber(root, "spec.ports[1].name")
	if linePort1 != 11 {
		t.Errorf("expected line for spec.ports[1].name to be 11, got %d", linePort1)
	}

	lineUnknown := FindLineNumber(root, "spec.nonexistent")
	if lineUnknown != 0 {
		t.Errorf("expected 0 for nonexistent path, got %d", lineUnknown)
	}
}
