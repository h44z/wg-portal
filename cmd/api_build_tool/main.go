package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/swaggo/swag"
	"github.com/swaggo/swag/gen"
)

// this replaces the call to: swag init --propertyStrategy pascalcase --parseDependency --parseInternal --generalInfo base.go
func main() {
	wd, err := os.Getwd() // should be the project root
	if err != nil {
		panic(err)
	}

	apiBasePath := filepath.Join(wd, "/internal/app/api")
	apis := []string{"v0"}

	hasError := false
	for _, apiVersion := range apis {
		apiPath := filepath.Join(apiBasePath, apiVersion, "handlers")

		apiVersion = strings.TrimLeft(apiVersion, "api-")
		log.Println("")
		log.Println("Generate swagger docs for API", apiVersion)
		log.Println("Api path:", apiPath)

		err := generateApi(apiBasePath, apiPath, apiVersion)
		if err != nil {
			hasError = true
			logrus.Errorf("failed to generate API docs for %s: %v", apiVersion, err)
		}

		log.Println("Generated swagger docs for API", apiVersion)
	}

	if hasError {
		os.Exit(1)
	}
}

func generateApi(basePath, apiPath, version string) error {
	err := gen.New().Build(&gen.Config{
		SearchDir:           apiPath,
		Excludes:            "",
		MainAPIFile:         "base.go",
		PropNamingStrategy:  swag.PascalCase,
		OutputDir:           filepath.Join(basePath, "core/assets/doc"),
		OutputTypes:         []string{"json", "yaml"},
		ParseVendor:         false,
		ParseDependency:     true,
		MarkdownFilesDir:    "",
		ParseInternal:       true,
		GeneratedTime:       false,
		CodeExampleFilesDir: "",
		ParseDepth:          3,
		InstanceName:        version,
	})
	if err != nil {
		return fmt.Errorf("swag failed: %w", err)
	}

	return nil
}
