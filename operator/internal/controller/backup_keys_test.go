package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestMissingRepoSecretKeys(t *testing.T) {
	cases := []struct {
		name string
		sec  *corev1.Secret
		want []string
	}{
		{
			name: "both present in Data",
			sec:  &corev1.Secret{Data: map[string][]byte{"repo": []byte("r"), "password": []byte("p")}},
			want: nil,
		},
		{
			name: "both present in StringData",
			sec:  &corev1.Secret{StringData: map[string]string{"repo": "r", "password": "p"}},
			want: nil,
		},
		{
			name: "missing repo (legacy url-only)",
			sec:  &corev1.Secret{Data: map[string][]byte{"url": []byte("r"), "password": []byte("p")}},
			want: []string{"repo"},
		},
		{
			name: "missing both",
			sec:  &corev1.Secret{Data: map[string][]byte{"other": []byte("x")}},
			want: []string{"repo", "password"},
		},
		{
			name: "empty repo value counts as missing",
			sec:  &corev1.Secret{Data: map[string][]byte{"repo": {}, "password": []byte("p")}},
			want: []string{"repo"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := missingRepoSecretKeys(tc.sec)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
