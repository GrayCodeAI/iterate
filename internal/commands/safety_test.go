package commands

import (
	"testing"
)

// ---------------------------------------------------------------------------
// cmdSafe
// ---------------------------------------------------------------------------

func TestCmdSafe_SetsTrue(t *testing.T) {
	safe := false
	ctx := Context{
		SafeMode: &safe,
		Parts:    []string{"/safe"},
	}
	result := cmdSafe(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if !safe {
		t.Error("expected SafeMode to be true after /safe")
	}
}

func TestCmdSafe_NilSafeMode(t *testing.T) {
	ctx := Context{
		SafeMode: nil,
		Parts:    []string{"/safe"},
	}
	result := cmdSafe(ctx)
	if !result.Handled {
		t.Error("expected command to be handled even with nil SafeMode")
	}
}

// ---------------------------------------------------------------------------
// cmdTrust
// ---------------------------------------------------------------------------

func TestCmdTrust_SetsFalse(t *testing.T) {
	safe := true
	ctx := Context{
		SafeMode: &safe,
		Parts:    []string{"/trust"},
	}
	result := cmdTrust(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if safe {
		t.Error("expected SafeMode to be false after /trust")
	}
}

func TestCmdTrust_NilSafeMode(t *testing.T) {
	ctx := Context{
		SafeMode: nil,
		Parts:    []string{"/trust"},
	}
	result := cmdTrust(ctx)
	if !result.Handled {
		t.Error("expected command to be handled even with nil SafeMode")
	}
}

// ---------------------------------------------------------------------------
// cmdAllow
// ---------------------------------------------------------------------------

func TestCmdAllow_WithArg(t *testing.T) {
	var allowedTool string
	ctx := Context{
		Parts: []string{"/allow", "bash"},
		State: StateAccessors{
			AllowTool: func(name string) {
				allowedTool = name
			},
		},
	}
	result := cmdAllow(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if allowedTool != "bash" {
		t.Errorf("expected tool 'bash' to be allowed, got %q", allowedTool)
	}
}

func TestCmdAllow_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/allow"},
	}
	result := cmdAllow(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdAllow_NilAllowTool(t *testing.T) {
	ctx := Context{
		Parts: []string{"/allow", "bash"},
		State: StateAccessors{
			AllowTool: nil,
		},
	}
	result := cmdAllow(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdDeny
// ---------------------------------------------------------------------------

func TestCmdDeny_WithArg(t *testing.T) {
	var deniedTool string
	ctx := Context{
		Parts: []string{"/deny", "write_file"},
		State: StateAccessors{
			DenyTool: func(name string) {
				deniedTool = name
			},
		},
	}
	result := cmdDeny(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if deniedTool != "write_file" {
		t.Errorf("expected tool 'write_file' to be denied, got %q", deniedTool)
	}
}

func TestCmdDeny_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/deny"},
	}
	result := cmdDeny(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdDeny_NilDenyTool(t *testing.T) {
	ctx := Context{
		Parts: []string{"/deny", "bash"},
		State: StateAccessors{
			DenyTool: nil,
		},
	}
	result := cmdDeny(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdConfig
// ---------------------------------------------------------------------------

func TestCmdConfig_WithSafeMode(t *testing.T) {
	safe := true
	ctx := Context{
		SafeMode: &safe,
		Parts:    []string{"/config"},
		State:    StateAccessors{},
	}
	result := cmdConfig(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdConfig_WithThinking(t *testing.T) {
	ctx := Context{
		Parts: []string{"/config"},
		State: StateAccessors{},
	}
	result := cmdConfig(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdConfig_WithDeniedTools(t *testing.T) {
	ctx := Context{
		Parts: []string{"/config"},
		State: StateAccessors{
			GetDeniedList: func() []string {
				return []string{"bash", "write_file"}
			},
		},
	}
	result := cmdConfig(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdConfig_EmptyDeniedList(t *testing.T) {
	ctx := Context{
		Parts: []string{"/config"},
		State: StateAccessors{
			GetDeniedList: func() []string {
				return nil
			},
		},
	}
	result := cmdConfig(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// RegisterSafetyCommands
// ---------------------------------------------------------------------------

func TestRegisterSafetyCommands(t *testing.T) {
	r := NewRegistry()
	RegisterSafetyCommands(r)

	expected := []string{"/safe", "/trust", "/allow", "/deny", "/config"}
	for _, name := range expected {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}
