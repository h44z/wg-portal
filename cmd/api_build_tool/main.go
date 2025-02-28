package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/swaggo/swag"
	"github.com/swaggo/swag/gen"
	"gopkg.in/yaml.v2"
)

var apiRootPath = "/internal/app/api"
var apiDocPath = "core/assets/doc"
var apiMkDocPath = "/docs/documentation/rest-api"

// this replaces the call to: swag init --propertyStrategy pascalcase --parseDependency --parseInternal --generalInfo base.go
func main() {
	wd, err := os.Getwd() // should be the project root
	if err != nil {
		panic(err)
	}

	apiBasePath := filepath.Join(wd, apiRootPath)
	apis := []string{"v0", "v1"}

	for _, apiVersion := range apis {
		apiPath := filepath.Join(apiBasePath, apiVersion, "handlers")

		apiVersion = strings.TrimLeft(apiVersion, "api-")
		log.Println("")
		log.Println("Generate swagger docs for API", apiVersion)
		log.Println("Api path:", apiPath)

		err := generateApi(apiBasePath, apiPath, apiVersion)
		if err != nil {
			log.Fatalf("failed to generate API docs for %s: %v", apiVersion, err)
		}

		// copy the latest version of the API docs for mkdocs
		if apiVersion == apis[len(apis)-1] {
			if err = copyDocForMkdocs(wd, apiBasePath, apiVersion); err != nil {
				log.Printf("failed to copy API docs for mkdocs: %v", err)
			} else {
				log.Println("Copied API docs " + apiVersion + " for mkdocs")
			}
		}

		log.Println("Generated swagger docs for API", apiVersion)
	}
}

func generateApi(basePath, apiPath, version string) error {
	err := gen.New().Build(&gen.Config{
		SearchDir:           apiPath,
		Excludes:            "",
		MainAPIFile:         "base.go",
		PropNamingStrategy:  swag.PascalCase,
		OutputDir:           filepath.Join(basePath, apiDocPath),
		OutputTypes:         []string{"json", "yaml"},
		ParseVendor:         false,
		ParseDependency:     3,
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

func copyDocForMkdocs(workingDir, basePath, version string) error {
	srcPath := filepath.Join(basePath, apiDocPath, fmt.Sprintf("%s_swagger.yaml", version))
	dstPath := filepath.Join(workingDir, apiMkDocPath, "swagger.yaml")

	// copy the file
	input, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("error while reading swagger doc: %w", err)
	}

	output, err := removeAuthorizeButton(input)
	if err != nil {
		return fmt.Errorf("error while removing authorize button: %w", err)
	}

	err = os.WriteFile(dstPath, output, 0644)
	if err != nil {
		return fmt.Errorf("error while writing swagger doc: %w", err)
	}

	return nil
}

func removeAuthorizeButton(input []byte) ([]byte, error) {
	var swagger map[string]any
	err := yaml.Unmarshal(input, &swagger)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling swagger file: %w", err)
	}

	delete(swagger, "securityDefinitions")

	output, err := yaml.Marshal(&swagger)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling swagger file: %w", err)
	}

	return output, nil
}
