package commands

// RegisterAll adds all command groups to the registry.
// This is the single entry point for wiring up all modular commands.
func RegisterAll(r *Registry) {
	RegisterSessionCommands(r)
	RegisterSafetyCommands(r)
	RegisterDevCommands(r)
	RegisterAutofixCommands(r)
	RegisterGitCommands(r)
	RegisterGitContextCommands(r)
	RegisterAgentCommands(r)
	RegisterEvolutionCommands(r)
	RegisterMemoryCommands(r)
	RegisterMemoryAnalyticsCommands(r)
	RegisterGitHubCommands(r)
	RegisterUtilityCommands(r)
	RegisterModeCommands(r)
	RegisterFileCommands(r)
	RegisterConfigCommands(r)
	RegisterProfileCommands(r)
	RegisterAnalysisCommands(r)
	RegisterBudgetCommands(r)
	RegisterTemplateCommands(r)
	RegisterSnippetCommands(r)
	RegisterASTCommands(r)
	RegisterLearningCommands(r)
	RegisterGitHooksCommands(r)
	RegisterContextTemplateCommands(r)
	RegisterDocsCommands(r)
}

// DefaultRegistry returns a fully populated registry.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	RegisterAll(r)
	return r
}
