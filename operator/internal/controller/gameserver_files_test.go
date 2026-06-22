package controller

import (
	"strings"
	"testing"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func TestBuildConfigInitContainer_Defaults(t *testing.T) {
	tmpl := &kestrelv1alpha1.GameTemplate{}
	c := buildConfigInitContainer(tmpl)

	if c.Name != "config-init" {
		t.Errorf("name = %q, want config-init", c.Name)
	}
	if c.Image != configInitImage {
		t.Errorf("image = %q, want pinned %q", c.Image, configInitImage)
	}
	if len(c.Args) != 1 || !strings.Contains(c.Args[0], configFilesStagingPath+"/*") {
		t.Errorf("args should copy from the staging glob, got %v", c.Args)
	}
	if !strings.Contains(c.Args[0], "'/data/'") {
		t.Errorf("args should copy into the default mount path, got %v", c.Args)
	}
	if len(c.VolumeMounts) != 2 {
		t.Fatalf("got %d volume mounts, want 2: %v", len(c.VolumeMounts), c.VolumeMounts)
	}
	staging, data := c.VolumeMounts[0], c.VolumeMounts[1]
	if staging.Name != "config-files" || staging.MountPath != configFilesStagingPath || !staging.ReadOnly {
		t.Errorf("staging mount = %+v, want read-only config-files at %s", staging, configFilesStagingPath)
	}
	if data.Name != "data" || data.MountPath != "/data" || data.ReadOnly {
		t.Errorf("data mount = %+v, want writable data at /data", data)
	}
}

func TestBuildConfigInitContainer_HonorsMountPath(t *testing.T) {
	tmpl := &kestrelv1alpha1.GameTemplate{
		Spec: kestrelv1alpha1.GameTemplateSpec{
			Storage: kestrelv1alpha1.GameStorageSpec{MountPath: "/world"},
		},
	}
	c := buildConfigInitContainer(tmpl)

	if !strings.Contains(c.Args[0], "'/world/'") {
		t.Errorf("args should copy into /world, got %v", c.Args)
	}
	if c.VolumeMounts[1].MountPath != "/world" {
		t.Errorf("data mount path = %q, want /world", c.VolumeMounts[1].MountPath)
	}
}
