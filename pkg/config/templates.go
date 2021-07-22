package config

import (
	"embed"
	"path/filepath"

	"github.com/harvester/harvester-installer/pkg/util"
)

const (
	templateFolder = "templates"
)

var (
	//go:embed templates/*
	templFS embed.FS
)

// render renders a template in the package templates folder (build-time)
func render(template string, context interface{}) (string, error) {
	templBytes, err := templFS.ReadFile(filepath.Join(templateFolder, template))
	if err != nil {
		return "", err
	}
	return util.RenderTemplate(string(templBytes), context)
}
