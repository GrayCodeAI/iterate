module github.com/GrayCodeAI/iterate

go 1.22

require (
	github.com/GrayCodeAI/iteragent v1.0.10
	github.com/google/go-github/v61 v61.0.0
	golang.org/x/oauth2 v0.20.0
)

require github.com/google/go-querystring v1.1.0 // indirect

replace github.com/GrayCodeAI/iteragent => ../iteragent
