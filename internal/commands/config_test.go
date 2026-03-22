package commands

import (
	"testing"
)

// ---------------------------------------------------------------------------
// cmdSet
// ---------------------------------------------------------------------------

func TestCmdSet_NilRuntimeConfig(t *testing.T) {
	ctx := Context{
		Parts:         []string{"/set"},
		RuntimeConfig: nil,
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true even with nil RuntimeConfig")
	}
}

func TestCmdSet_ShowCurrentConfig(t *testing.T) {
	temp := float32(0.7)
	maxt := 4096
	ctx := Context{
		Parts: []string{"/set"},
		RuntimeConfig: &RuntimeConfig{
			Temperature: &temp,
			MaxTokens:   &maxt,
		},
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSet_ShowCurrentConfig_Defaults(t *testing.T) {
	ctx := Context{
		Parts:         []string{"/set"},
		RuntimeConfig: &RuntimeConfig{},
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSet_Temperature(t *testing.T) {
	rc := &RuntimeConfig{}
	ctx := Context{
		Parts:         []string{"/set", "temperature", "0.9"},
		RuntimeConfig: rc,
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if rc.Temperature == nil {
		t.Fatal("expected Temperature to be set")
	}
	if *rc.Temperature < 0.89 || *rc.Temperature > 0.91 {
		t.Errorf("expected temperature ~0.9, got %f", *rc.Temperature)
	}
}

func TestCmdSet_Temperature_Alias(t *testing.T) {
	rc := &RuntimeConfig{}
	ctx := Context{
		Parts:         []string{"/set", "temp", "0.5"},
		RuntimeConfig: rc,
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if rc.Temperature == nil {
		t.Fatal("expected Temperature to be set")
	}
}

func TestCmdSet_MaxTokens(t *testing.T) {
	rc := &RuntimeConfig{}
	ctx := Context{
		Parts:         []string{"/set", "max_tokens", "8192"},
		RuntimeConfig: rc,
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if rc.MaxTokens == nil {
		t.Fatal("expected MaxTokens to be set")
	}
	if *rc.MaxTokens != 8192 {
		t.Errorf("expected max_tokens=8192, got %d", *rc.MaxTokens)
	}
}

func TestCmdSet_MaxTokens_Aliases(t *testing.T) {
	for _, alias := range []string{"max-tokens", "tokens"} {
		rc := &RuntimeConfig{}
		ctx := Context{
			Parts:         []string{"/set", alias, "2048"},
			RuntimeConfig: rc,
		}
		cmdSet(ctx)
		if rc.MaxTokens == nil || *rc.MaxTokens != 2048 {
			t.Errorf("alias %q: expected max_tokens=2048", alias)
		}
	}
}

func TestCmdSet_Reset(t *testing.T) {
	temp := float32(0.9)
	maxt := 4096
	rc := &RuntimeConfig{
		Temperature: &temp,
		MaxTokens:   &maxt,
	}
	ctx := Context{
		Parts:         []string{"/set", "reset", "placeholder"},
		RuntimeConfig: rc,
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if rc.Temperature != nil {
		t.Error("expected Temperature to be nil after reset")
	}
	if rc.MaxTokens != nil {
		t.Error("expected MaxTokens to be nil after reset")
	}
}

func TestCmdSet_UnknownSetting(t *testing.T) {
	rc := &RuntimeConfig{}
	ctx := Context{
		Parts:         []string{"/set", "unknown_key", "value"},
		RuntimeConfig: rc,
	}
	result := cmdSet(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdAlias
// ---------------------------------------------------------------------------

func TestCmdAlias_NilCallbacks(t *testing.T) {
	ctx := Context{
		Parts:  []string{"/alias"},
		Config: ConfigCallbacks{},
	}
	result := cmdAlias(ctx)
	if !result.Handled {
		t.Error("expected handled=true even with nil callbacks")
	}
}

func TestCmdAlias_ShowEmpty(t *testing.T) {
	aliases := map[string]string{}
	ctx := Context{
		Parts: []string{"/alias"},
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string { return aliases },
			SaveAliases: func(m map[string]string) {},
		},
	}
	result := cmdAlias(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdAlias_ShowList(t *testing.T) {
	aliases := map[string]string{"g": "/git status", "t": "/test"}
	ctx := Context{
		Parts: []string{"/alias"},
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string { return aliases },
			SaveAliases: func(m map[string]string) {},
		},
	}
	result := cmdAlias(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdAlias_UsageWhenMissingCommand(t *testing.T) {
	aliases := map[string]string{}
	ctx := Context{
		Parts: []string{"/alias", "myalias"},
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string { return aliases },
			SaveAliases: func(m map[string]string) {},
		},
	}
	result := cmdAlias(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdAlias_Add(t *testing.T) {
	aliases := map[string]string{}
	var saved map[string]string
	ctx := Context{
		Parts: []string{"/alias", "g", "/git", "status"},
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string { return aliases },
			SaveAliases: func(m map[string]string) { saved = m },
		},
	}
	result := cmdAlias(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if saved == nil {
		t.Fatal("expected SaveAliases to be called")
	}
	if saved["g"] != "/git status" {
		t.Errorf("expected alias 'g' -> '/git status', got %q", saved["g"])
	}
}

func TestCmdAlias_Delete(t *testing.T) {
	aliases := map[string]string{"g": "/git status", "t": "/test"}
	var saved map[string]string
	ctx := Context{
		Parts: []string{"/alias", "g", "delete"},
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string { return aliases },
			SaveAliases: func(m map[string]string) { saved = m },
		},
	}
	result := cmdAlias(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if saved == nil {
		t.Fatal("expected SaveAliases to be called")
	}
	if _, exists := saved["g"]; exists {
		t.Error("expected alias 'g' to be deleted")
	}
	if _, exists := saved["t"]; !exists {
		t.Error("expected alias 't' to still exist")
	}
}

// ---------------------------------------------------------------------------
// cmdNotify
// ---------------------------------------------------------------------------

func TestCmdNotify_NilState(t *testing.T) {
	ctx := Context{
		NotifyEnabled: nil,
	}
	result := cmdNotify(ctx)
	if !result.Handled {
		t.Error("expected handled=true even with nil NotifyEnabled")
	}
}

func TestCmdNotify_Toggle(t *testing.T) {
	enabled := false
	ctx := Context{
		NotifyEnabled: &enabled,
	}
	result := cmdNotify(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if !*ctx.NotifyEnabled {
		t.Error("expected NotifyEnabled to be toggled to true")
	}

	// Toggle again
	cmdNotify(ctx)
	if *ctx.NotifyEnabled {
		t.Error("expected NotifyEnabled to be toggled back to false")
	}
}

// ---------------------------------------------------------------------------
// cmdDebug
// ---------------------------------------------------------------------------

func TestCmdDebug_NilState(t *testing.T) {
	ctx := Context{DebugMode: nil}
	result := cmdDebug(ctx)
	if !result.Handled {
		t.Error("expected handled=true even with nil DebugMode")
	}
}

func TestCmdDebug_Toggle(t *testing.T) {
	debug := false
	ctx := Context{DebugMode: &debug}
	result := cmdDebug(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if !*ctx.DebugMode {
		t.Error("expected DebugMode to be toggled to true")
	}

	cmdDebug(ctx)
	if *ctx.DebugMode {
		t.Error("expected DebugMode to be toggled back to false")
	}
}

// ---------------------------------------------------------------------------
// cmdAliases
// ---------------------------------------------------------------------------

func TestCmdAliases_NilCallback(t *testing.T) {
	ctx := Context{
		Config: ConfigCallbacks{},
	}
	result := cmdAliases(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdAliases_Empty(t *testing.T) {
	ctx := Context{
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string { return map[string]string{} },
		},
	}
	result := cmdAliases(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdAliases_WithAliases(t *testing.T) {
	ctx := Context{
		Config: ConfigCallbacks{
			LoadAliases: func() map[string]string {
				return map[string]string{"g": "/git status", "t": "/test"}
			},
		},
	}
	result := cmdAliases(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}
