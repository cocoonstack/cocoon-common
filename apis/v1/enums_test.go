package v1

import "testing"

func TestAgentModeIsValid(t *testing.T) {
	cases := []struct {
		in   AgentMode
		want bool
	}{
		{AgentModeClone, true},
		{AgentModeRun, true},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("AgentMode(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestToolboxModeIsValid(t *testing.T) {
	cases := []struct {
		in   ToolboxMode
		want bool
	}{
		{ToolboxModeRun, true},
		{ToolboxModeClone, true},
		{ToolboxModeStatic, true},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("ToolboxMode(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestOSTypeIsValid(t *testing.T) {
	cases := []struct {
		in   OSType
		want bool
	}{
		{OSLinux, true},
		{OSWindows, true},
		{OSAndroid, true},
		{"", false},
		{"freebsd", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("OSType(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSnapshotPolicyIsValid(t *testing.T) {
	cases := []struct {
		in   SnapshotPolicy
		want bool
	}{
		{SnapshotPolicyAlways, true},
		{SnapshotPolicyMainOnly, true},
		{SnapshotPolicyNever, true},
		{"", false},
		{"sometimes", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("SnapshotPolicy(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestConnTypeIsValid(t *testing.T) {
	cases := []struct {
		in   ConnType
		want bool
	}{
		{ConnTypeSSH, true},
		{ConnTypeRDP, true},
		{ConnTypeVNC, true},
		{ConnTypeADB, true},
		{"", false},
		{"telnet", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("ConnType(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestBackendIsValid(t *testing.T) {
	cases := []struct {
		in   Backend
		want bool
	}{
		{BackendCloudHypervisor, true},
		{BackendFirecracker, true},
		{"", false},
		{"qemu", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("Backend(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestEnumDefaults(t *testing.T) {
	t.Run("AgentMode", func(t *testing.T) {
		if got := AgentMode("").Default(); got != AgentModeClone {
			t.Errorf("empty default = %q, want %q", got, AgentModeClone)
		}
		if got := AgentModeRun.Default(); got != AgentModeRun {
			t.Errorf("set value should pass through, got %q", got)
		}
	})
	t.Run("ToolboxMode", func(t *testing.T) {
		if got := ToolboxMode("").Default(); got != ToolboxModeRun {
			t.Errorf("empty default = %q, want %q", got, ToolboxModeRun)
		}
		if got := ToolboxModeStatic.Default(); got != ToolboxModeStatic {
			t.Errorf("set value should pass through, got %q", got)
		}
	})
	t.Run("OSType", func(t *testing.T) {
		if got := OSType("").Default(); got != OSLinux {
			t.Errorf("empty default = %q, want %q", got, OSLinux)
		}
		if got := OSWindows.Default(); got != OSWindows {
			t.Errorf("set value should pass through, got %q", got)
		}
	})
	t.Run("SnapshotPolicy", func(t *testing.T) {
		if got := SnapshotPolicy("").Default(); got != SnapshotPolicyAlways {
			t.Errorf("empty default = %q, want %q", got, SnapshotPolicyAlways)
		}
		if got := SnapshotPolicyNever.Default(); got != SnapshotPolicyNever {
			t.Errorf("set value should pass through, got %q", got)
		}
	})
	t.Run("Backend", func(t *testing.T) {
		if got := Backend("").Default(); got != BackendCloudHypervisor {
			t.Errorf("empty default = %q, want %q", got, BackendCloudHypervisor)
		}
		if got := BackendFirecracker.Default(); got != BackendFirecracker {
			t.Errorf("set value should pass through, got %q", got)
		}
	})
	t.Run("ConnType", func(t *testing.T) {
		// ConnType.Default is a pass-through: empty stays empty so
		// meta.ConnectionType can infer from OS/runtime instead.
		if got := ConnType("").Default(); got != "" {
			t.Errorf("empty should pass through unchanged, got %q", got)
		}
		if got := ConnTypeRDP.Default(); got != ConnTypeRDP {
			t.Errorf("set value should pass through, got %q", got)
		}
	})
}
