package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/GrayCodeAI/iterate/internal/commands"
)

// mcpServersPath returns the path to the persisted MCP server list.
func mcpServersPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "mcp_servers.json")
}

// loadMCPServers reads the persisted MCP server list from disk.
func loadMCPServers() []commands.MCPServerEntry {
	data, err := os.ReadFile(mcpServersPath())
	if err != nil {
		return nil
	}
	var servers []commands.MCPServerEntry
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil
	}
	return servers
}

// saveMCPServers writes the MCP server list to disk.
func saveMCPServers(servers []commands.MCPServerEntry) {
	if err := os.MkdirAll(filepath.Dir(mcpServersPath()), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(mcpServersPath(), data, 0o644); err != nil {
		slog.Warn("failed to write MCP servers file", "err", err)
	}
}

// mcpJSONEntry is the shape of an entry in .mcp.json.
type mcpJSONEntry struct {
	Name    string   `json:"name"`
	URL     string   `json:"url,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// discoverMCPServers reads .mcp.json from the repo root and merges any
// new entries into the persisted list. Entries already present (by name)
// are not overwritten. Returns the number of newly added servers.
func discoverMCPServers(absRepo string) int {
	mcpFile := filepath.Join(absRepo, ".mcp.json")
	data, err := os.ReadFile(mcpFile)
	if err != nil {
		return 0 // file absent — that's fine
	}

	// Support both top-level array and {servers:[...]} object.
	var discovered []mcpJSONEntry
	if err := json.Unmarshal(data, &discovered); err != nil {
		// Try object wrapper: {"servers": [...]}
		var wrapper struct {
			Servers []mcpJSONEntry `json:"servers"`
		}
		if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
			fmt.Fprintf(os.Stderr, "warn: .mcp.json parse error: %v\n", err)
			return 0
		}
		discovered = wrapper.Servers
	}

	if len(discovered) == 0 {
		return 0
	}

	existing := loadMCPServers()
	existingNames := make(map[string]bool, len(existing))
	for _, s := range existing {
		existingNames[s.Name] = true
	}

	added := 0
	for _, d := range discovered {
		if d.Name == "" || existingNames[d.Name] {
			continue
		}
		existing = append(existing, commands.MCPServerEntry{
			Name:    d.Name,
			URL:     d.URL,
			Command: d.Command,
			Args:    d.Args,
		})
		existingNames[d.Name] = true
		added++
	}

	if added > 0 {
		saveMCPServers(existing)
	}
	return added
}
