package ociutil

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantTag  string
	}{
		{"repo", "repo", "latest"},
		{"repo:v1", "repo", "v1"},
		{"ns/repo:v1.2", "ns/repo", "v1.2"},
		{"ghcr.io/cocoonstack/cocoon/ubuntu:24.04", "ghcr.io/cocoonstack/cocoon/ubuntu", "24.04"},
		{":tag", ":tag", "latest"},
		{"", "", "latest"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			name, tag := ParseRef(tt.ref)
			if name != tt.wantName || tag != tt.wantTag {
				t.Errorf("ParseRef(%q) = (%q, %q), want (%q, %q)", tt.ref, name, tag, tt.wantName, tt.wantTag)
			}
		})
	}
}

func TestIsRelativeRef(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"repo", true},
		{"repo:v1", true},
		{"ns/repo:v1.2-rc", true},
		{"ghcr.io/cocoonstack/cocoon/ubuntu:24.04", true},
		{"snap_shot-x:latest", true},

		{"registry:5000/repo:tag", false},
		{"registry:5000/repo", false},
		{"repo@sha256:deadbeef", false},
		{"https://cloud-images.ubuntu.com/noble.img", false},
		{"Repo:tag", false},
		{"repo:", false},
		{":tag", false},
		{"a:b:c", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			if got := IsRelativeRef(tt.ref); got != tt.want {
				t.Errorf("IsRelativeRef(%q) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}
