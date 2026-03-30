package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

type MCPConfig struct {
	Enabled       bool              `json:"enabled"`
	Servers       []MCPServerConfig `json:"servers"`
	Timeout       time.Duration     `json:"timeout"`
	RetryAttempts int               `json:"retry_attempts"`
}

type MCPServerConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	Transport   string            `json:"transport"` // stdio, http
	URL         string            `json:"url"`
	AutoConnect bool              `json:"auto_connect"`
}

type MCPClient struct {
	config      MCPConfig
	clients     map[string]*MCPConnection
	mu          sync.RWMutex
	repoPath    string
	logger      *slog.Logger
	initialized bool
}

type MCPConnection struct {
	server     MCPServerConfig
	process    *exec.Cmd
	stdin      *os.File
	stdout     *os.File
	requestID  int
	mu         sync.Mutex
	connected  bool
	lastActive time.Time
}

var globalMCPClient *MCPClient
var mcpOnce sync.Once

func GetMCPClient(repoPath string, logger *slog.Logger) *MCPClient {
	mcpOnce.Do(func() {
		globalMCPClient = &MCPClient{
			clients:  make(map[string]*MCPConnection),
			repoPath: repoPath,
			logger:   logger,
		}
	})
	return globalMCPClient
}

func (m *MCPClient) LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read MCP config: %w", err)
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse MCP config: %w", err)
	}

	m.config = config
	return nil
}

func (m *MCPClient) Init(ctx context.Context) error {
	if m.initialized {
		return nil
	}

	if !m.config.Enabled {
		m.logger.Info("MCP is disabled")
		return nil
	}

	m.logger.Info("Initializing MCP clients", "servers", len(m.config.Servers))

	for _, server := range m.config.Servers {
		if err := m.connectServer(ctx, server); err != nil {
			m.logger.Warn("Failed to connect MCP server", "server", server.Name, "err", err)
			continue
		}
	}

	m.initialized = true
	return nil
}

func (m *MCPClient) connectServer(ctx context.Context, server MCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[server.Name]; exists {
		return nil
	}

	conn := &MCPConnection{
		server:     server,
		lastActive: time.Now(),
	}

	if server.Transport == "stdio" {
		if err := m.startStdioServer(ctx, conn); err != nil {
			return fmt.Errorf("failed to start stdio server: %w", err)
		}
	}

	m.clients[server.Name] = conn
	m.logger.Info("Connected to MCP server", "server", server.Name)

	return nil
}

func (m *MCPClient) startStdioServer(ctx context.Context, conn *MCPConnection) error {
	cmd := exec.CommandContext(ctx, conn.server.Command, conn.server.Args...)
	cmd.Dir = m.repoPath

	if conn.server.Env != nil {
		for k, v := range conn.server.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	conn.process = cmd
	conn.stdin = stdin.(*os.File)
	conn.stdout = stdout.(*os.File)
	conn.connected = true

	go m.readServerMessages(conn)

	initializeReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "iterate",
				"version": "1.0.0",
			},
		},
	}

	if _, err := m.sendRequest(conn, initializeReq); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	return nil
}

func (m *MCPClient) readServerMessages(conn *MCPConnection) {
	decoder := json.NewDecoder(conn.stdout)
	for {
		var msg MCPMessage
		if err := decoder.Decode(&msg); err != nil {
			conn.connected = false
			m.logger.Warn("MCP server message decode error", "server", conn.server.Name, "err", err)
			break
		}
		conn.lastActive = time.Now()
	}
}

func (m *MCPClient) sendRequest(conn *MCPConnection, req MCPRequest) ([]byte, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !conn.connected {
		return nil, fmt.Errorf("MCP server not connected: %s", conn.server.Name)
	}

	conn.requestID++
	req.ID = conn.requestID

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err := conn.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	conn.lastActive = time.Now()
	return data, nil
}

func (m *MCPClient) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	conn, ok := m.clients[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP server not found: %s", serverName)
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	_, err := m.sendRequest(conn, req)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (m *MCPClient) ListTools(serverName string) ([]MCPTool, error) {
	m.mu.RLock()
	conn, ok := m.clients[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server not found: %s", serverName)
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
	}

	_, err := m.sendRequest(conn, req)
	if err != nil {
		return nil, err
	}

	return []MCPTool{}, nil
}

func (m *MCPClient) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.clients {
		if conn.process != nil && conn.process.Process != nil {
			conn.process.Process.Kill()
		}
		delete(m.clients, name)
	}

	m.initialized = false
}

func (m *MCPClient) GetAvailableTools() []iteragent.Tool {
	var tools []iteragent.Tool

	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, conn := range m.clients {
		if !conn.connected {
			continue
		}

		mcpTools, err := m.ListTools(name)
		if err != nil {
			continue
		}

		for _, t := range mcpTools {
			tool := iteragent.Tool{
				Name:        fmt.Sprintf("mcp_%s_%s", name, t.Name),
				Description: t.Description,
			}
			tools = append(tools, tool)
		}
	}

	return tools
}

func LoadMCPConfigFromDefaultLocations(repoPath string) (*MCPConfig, error) {
	locations := []string{
		filepath.Join(repoPath, ".iterate", "mcp.json"),
		filepath.Join(repoPath, "mcp.json"),
		filepath.Join(os.Getenv("HOME"), ".config", "iterate", "mcp.json"),
	}

	for _, loc := range locations {
		data, err := os.ReadFile(loc)
		if err == nil {
			var config MCPConfig
			if err := json.Unmarshal(data, &config); err == nil {
				return &config, nil
			}
		}
	}

	return &MCPConfig{Enabled: false}, nil
}

func (e *Engine) initMCP(ctx context.Context) error {
	config, err := LoadMCPConfigFromDefaultLocations(e.repoPath)
	if err != nil {
		return err
	}

	mcpClient := GetMCPClient(e.repoPath, e.logger)
	mcpClient.config = *config

	return mcpClient.Init(ctx)
}

type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema string `json:"inputSchema"`
}

func DefaultMCPConfig() MCPConfig {
	return MCPConfig{
		Enabled:       false,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		Servers:       []MCPServerConfig{},
	}
}

func CreateMCPConfigFile(repoPath string, servers []MCPServerConfig) error {
	config := MCPConfig{
		Enabled:       true,
		Servers:       servers,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	configPath := filepath.Join(repoPath, ".iterate", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write MCP config: %w", err)
	}

	return nil
}

func GetCommonMCPServers() []MCPServerConfig {
	var servers []MCPServerConfig

	if path, err := exec.LookPath("npx"); err == nil {
		servers = append(servers, MCPServerConfig{
			Name:        "filesystem",
			Command:     path,
			Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", os.Getenv("HOME")},
			Transport:   "stdio",
			AutoConnect: true,
		})
	}

	if path, err := exec.LookPath("uvx"); err == nil {
		servers = append(servers, MCPServerConfig{
			Name:        "brave-search",
			Command:     path,
			Args:        []string{"mcp-server-brave-search", "--api-key", os.Getenv("BRAVE_API_KEY")},
			Transport:   "stdio",
			AutoConnect: false,
		})
	}

	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		servers = append(servers, MCPServerConfig{
			Name:        "postgres",
			Command:     "docker",
			Args:        []string{"run", "--rm", "-i", "mcp/postgres"},
			Env:         map[string]string{"DATABASE_URL": os.Getenv("DATABASE_URL")},
			Transport:   "stdio",
			AutoConnect: false,
		})
	}

	return servers
}

func (m *MCPClient) HealthCheck() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make(map[string]bool)
	for name, conn := range m.clients {
		health[name] = conn.connected && time.Since(conn.lastActive) < 5*time.Minute
	}

	return health
}

func (m *MCPClient) ReconnectServer(ctx context.Context, serverName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, exists := m.clients[serverName]; exists {
		if conn.process != nil && conn.process.Process != nil {
			conn.process.Process.Kill()
		}
		delete(m.clients, serverName)
	}

	for _, server := range m.config.Servers {
		if server.Name == serverName {
			conn := &MCPConnection{
				server:     server,
				lastActive: time.Now(),
			}

			if server.Transport == "stdio" {
				if err := m.startStdioServer(ctx, conn); err != nil {
					return err
				}
			}

			m.clients[serverName] = conn
			return nil
		}
	}

	return fmt.Errorf("server not found: %s", serverName)
}

func (e *Engine) GetMCPTools() []iteragent.Tool {
	mcpClient := GetMCPClient(e.repoPath, e.logger)
	return mcpClient.GetAvailableTools()
}

func (e *Engine) CallMCPTool(ctx context.Context, server, tool string, args map[string]interface{}) (string, error) {
	mcpClient := GetMCPClient(e.repoPath, e.logger)
	return mcpClient.CallTool(ctx, server, tool, args)
}

func FindMCPConfigFiles(repoPath string) []string {
	var configs []string

	patterns := []string{
		".mcp.json",
		"mcp.json",
		".iterate/mcp.json",
		".config/iterate/mcp.json",
	}

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, ".config") {
			pattern = filepath.Join(os.Getenv("HOME"), pattern)
		} else {
			pattern = filepath.Join(repoPath, pattern)
		}

		matches, _ := filepath.Glob(pattern)
		configs = append(configs, matches...)
	}

	return configs
}
