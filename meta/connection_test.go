package meta

import "testing"

func TestConnectionType(t *testing.T) {
	cases := []struct {
		name       string
		osType     string
		hasVNCPort bool
		override   string
		want       string
	}{
		{name: "vnc wins", osType: "windows", hasVNCPort: true, want: "vnc"},
		{name: "windows", osType: "windows", want: "rdp"},
		{name: "android", osType: "android", want: "adb"},
		{name: "default", osType: "linux", want: "ssh"},
		{name: "override beats os", osType: "linux", override: "rdp", want: "rdp"},
		{name: "override beats vnc port", osType: "linux", hasVNCPort: true, override: "rdp", want: "rdp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConnectionType(tc.osType, tc.hasVNCPort, tc.override); got != tc.want {
				t.Fatalf("connection type mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}
