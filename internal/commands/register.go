package commands

// RegisterAll adds all command groups to the registry.
// This is the single entry point for wiring up all modular commands.
func RegisterAll(r *Registry) {
	RegisterSessionCommands(r)
	RegisterSafetyCommands(r)
	RegisterDevCommands(r)
	RegisterGitCommands(r)
	RegisterAgentCommands(r)
	RegisterEvolutionCommands(r)
	RegisterMemoryCommands(r)
	RegisterGitHubCommands(r)
	RegisterUtilityCommands(r)
	RegisterModeCommands(r)
	RegisterFileCommands(r)
	RegisterConfigCommands(r)
	RegisterAnalysisCommands(r)
}

// DefaultRegistry returns a fully populated registry.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	RegisterAll(r)
	return r
}
