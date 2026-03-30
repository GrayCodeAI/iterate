package commands

import (
	"bytes"
	"testing"
)

func TestHelpCommand_Execute(t *testing.T) {
	h := NewHelp(nil)
	buf := &bytes.Buffer{}
	
	err := h.Execute(buf, []string{})
	
	if err != nil {
		t.Errorf("Execute() returned error: %v", err)
	}
	
	output := buf.String()
	
	// Check that all commands are mentioned
	expectedCommands := []string{"help", "run", "list", "show", "exit"}
	for _, cmd := range expectedCommands {
		if !bytes.Contains([]byte(output), []byte(cmd)) {
			t.Errorf("Help output missing command %q", cmd)
		}
	}
}

func TestHelpCommand_Name(t *testing.T) {
	h := NewHelp(nil)
	if h.Name() != "help" {
		t.Errorf("Name() = %q, want %q", h.Name(), "help")
	}
}

func TestHelpCommand_Description(t *testing.T) {
	h := NewHelp(nil)
	if h.Description() == "" {
		t.Error("Description() should not be empty")
	}
}
