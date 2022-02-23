package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tcnksm/go-input"
	"gopkg.in/yaml.v3"
)

type Template struct {
	Name         string     `json:"name" yaml:"name"`
	Description  string     `json:"description" yaml:"description"`
	Variables    []Variable `json:"variables" yaml:"variables"`
	SpecLocation string     `json:"specLocation" yaml:"specLocation"`
}

type Variable struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Prompt      string `json:"prompt" yaml:"prompt"`
	Default     string `json:"default" yaml:"default"`
	Required    bool   `json:"required" yaml:"required"`
	Loop        bool   `json:"loop" yaml:"loop"`
}

type NewCmd struct {
	Template  string            `arg:"" help:"The template for the project to create."`
	Dir       string            `arg:"" help:"The project directory"`
	Spec      string            `type:"existingfile" help:"An optional specification file to copy into the project"`
	Variables map[string]string `arg:"" help:"Variables to pass to the template." optional:""`
}

var moduleAliases = map[string]string{
	"module": "@apexlang/core/module",
}

func AddModuleAliases(aliases map[string]string) {
	for name, path := range aliases {
		moduleAliases[name] = path
	}
}

func (c *NewCmd) Run(ctx *Context) error {
	if strings.Contains(c.Template, "..") {
		return fmt.Errorf("invalid template %s", c.Template)
	}

	homeDir, err := getHomeDirectory()
	if err != nil {
		return err
	}

	if translation, exists := moduleAliases[c.Template]; exists {
		c.Template = translation
	}
	c.Template = strings.ReplaceAll(c.Template, "/", string(filepath.Separator))

	templatePath := filepath.Join(homeDir, "templates", c.Template)
	templateDir, err := os.Stat(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template %s is not installed", c.Template)
		}
		return err
	}
	if !templateDir.IsDir() {
		return fmt.Errorf("%s is not a template directory", templatePath)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, c.Dir)
	if err != nil {
		return err
	}

	fmt.Printf("Creating project directory %s\n", projectPath)
	if err = os.MkdirAll(projectPath, 0777); err != nil {
		return err
	}

	if c.Variables == nil {
		c.Variables = map[string]string{}
	}
	// project name defaults to directory name
	if _, ok := c.Variables["name"]; !ok {
		name := filepath.Base(projectPath)
		c.Variables["name"] = name
	}

	templateBytes, err := os.ReadFile(filepath.Join(templatePath, ".template"))
	if err != nil {
		return err
	}

	var template Template
	if err = yaml.Unmarshal(templateBytes, &template); err != nil {
		return err
	}

	ui := &input.UI{
		Writer: os.Stdout,
		Reader: os.Stdin,
	}

	for _, variable := range template.Variables {
		if _, ok := c.Variables[variable.Name]; !ok {
			value, err := ui.Ask(variable.Prompt, &input.Options{
				Default:   variable.Default,
				Required:  variable.Required,
				Loop:      variable.Loop,
				HideOrder: true,
			})
			if err != nil {
				return err
			}
			c.Variables[variable.Name] = value
		}
	}

	err = c.copy(templatePath, projectPath, c.Variables)
	if err != nil {
		return err
	}

	if c.Spec != "" {
		if template.SpecLocation == "" {
			template.SpecLocation = "spec.apex"
		}
		specFilename := filepath.Join(projectPath, filepath.Clean(template.SpecLocation))
		specBytes, err := os.ReadFile(c.Spec)
		if err != nil {
			return err
		}
		err = os.WriteFile(specFilename, specBytes, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *NewCmd) copy(source, destination string, variables map[string]string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, ferr error) error {
		var relPath string = strings.Replace(path, source, "", 1)
		if relPath == "" {
			return nil
		}

		sourcePath := filepath.Join(source, relPath)
		stat, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			dstPath := filepath.Join(destination, relPath)
			dstPath, err = injectPathVariables(dstPath, variables)
			if err != nil {
				return err
			}
			return os.Mkdir(dstPath, stat.Mode())
		} else {
			base := filepath.Base(sourcePath)
			if base == ".keep" || base == ".gitkeep" ||
				base == ".template" || strings.HasPrefix(base, ".git") {
				return nil
			}

			data, err := os.ReadFile(sourcePath)
			if err != nil {
				return err
			}

			if filepath.Ext(relPath) == ".tmpl" {
				tmpl, err := template.New(relPath).Parse(string(data))
				if err != nil {
					return err
				}
				var buf bytes.Buffer
				if err = tmpl.Execute(&buf, c.Variables); err != nil {
					return err
				}

				data = buf.Bytes()
				relPath = relPath[:len(relPath)-5]
			}

			dstPath := filepath.Join(destination, relPath)
			dstPath, err = injectPathVariables(dstPath, variables)
			if err != nil {
				return err
			}
			return os.WriteFile(dstPath, data, stat.Mode())
		}
	})
}

func injectPathVariables(dstPath string, variables map[string]string) (string, error) {
	tmpl, err := template.New("destPath").Parse(dstPath)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, variables); err != nil {
		return "", err
	}
	return buf.String(), nil
}
