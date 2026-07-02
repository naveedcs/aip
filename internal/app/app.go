package app

import (
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/tools"
)

type App struct {
	Paths paths.Paths
	Tools []tools.Tool
}

func New(root string) App {
	return App{
		Paths: paths.ForRoot(root),
		Tools: tools.All(),
	}
}
