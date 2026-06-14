package handlers

import "testing"

func TestSourceRequest_Validate(t *testing.T) {
	okOCI := &ociSourceSpec{URL: "ghcr.io/x", Modules: []nameRef{{Name: "m"}}}
	cases := []struct {
		name string
		in   sourceRequest
		ok   bool
	}{
		{"oci ok", sourceRequest{Type: "oci", OCI: okOCI}, true},
		{"oci missing config", sourceRequest{Type: "oci"}, false},
		{"oci missing url and modules", sourceRequest{Type: "oci", OCI: &ociSourceSpec{}}, false},
		{"git ok", sourceRequest{Type: "git", Git: &gitSourceSpec{URL: "https://x"}}, true},
		{"git missing config", sourceRequest{Type: "git"}, false},
		{"git missing url", sourceRequest{Type: "git", Git: &gitSourceSpec{}}, false},
		{"http ok", sourceRequest{Type: "http", HTTP: &httpSourceSpec{URL: "https://x"}}, true},
		{"http missing config", sourceRequest{Type: "http"}, false},
		{"http missing url", sourceRequest{Type: "http", HTTP: &httpSourceSpec{}}, false},
		{"local ok", sourceRequest{Type: "local", Local: &localSourceSpec{Path: "p"}}, true},
		{"local missing config", sourceRequest{Type: "local"}, false},
		{"upload needs no config", sourceRequest{Type: "upload"}, true},
		{"unknown type", sourceRequest{Type: "ftp"}, false},
		{
			"config for the wrong type is rejected",
			sourceRequest{Type: "oci", OCI: okOCI, Git: &gitSourceSpec{URL: "https://x"}},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.validate()
			if (err == nil) != tc.ok {
				t.Fatalf("validate() err=%v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestSourceRequest_Spec(t *testing.T) {
	t.Run("oci with every option", func(t *testing.T) {
		in := sourceRequest{
			Type: "oci", RefreshInterval: "10m", Allow: []string{"m*"},
			OCI: &ociSourceSpec{
				URL: "ghcr.io/x", Modules: []nameRef{{Name: "m"}},
				Insecure: true, PullSecretRef: &nameRef{Name: "ps"},
			},
		}
		spec := in.spec()
		if spec["type"] != "oci" || spec["refreshInterval"] != "10m" || spec["allow"] == nil {
			t.Fatalf("top-level spec = %v", spec)
		}
		oci := spec["oci"].(map[string]any)
		if oci["url"] != "ghcr.io/x" || oci["insecure"] != true || oci["pullSecretRef"] == nil {
			t.Fatalf("oci spec = %v", oci)
		}
	})

	t.Run("git with ref, subPath and secret", func(t *testing.T) {
		in := sourceRequest{Type: "git", Git: &gitSourceSpec{
			URL: "https://x", Ref: "main", SubPath: "mods", SecretRef: &nameRef{Name: "s"},
		}}
		git := in.spec()["git"].(map[string]any)
		if git["ref"] != "main" || git["subPath"] != "mods" || git["secretRef"] == nil {
			t.Fatalf("git spec = %v", git)
		}
	})

	t.Run("http with insecure and secret", func(t *testing.T) {
		in := sourceRequest{Type: "http", HTTP: &httpSourceSpec{
			URL: "https://x", Insecure: true, SecretRef: &nameRef{Name: "s"},
		}}
		hs := in.spec()["http"].(map[string]any)
		if hs["insecure"] != true || hs["secretRef"] == nil {
			t.Fatalf("http spec = %v", hs)
		}
	})

	t.Run("local with path", func(t *testing.T) {
		in := sourceRequest{Type: "local", Local: &localSourceSpec{Path: "/data/mods"}}
		local := in.spec()["local"].(map[string]any)
		if local["path"] != "/data/mods" {
			t.Fatalf("local spec = %v", local)
		}
	})
}
