/*
Copyright 2022 The Apex Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tcnksm/go-input"
	"gopkg.in/yaml.v3"
)

type InitCmd struct {
	fromNew   bool
	Template  string            `arg:"" help:"The template for the project to create."`
	Dir       string            `type:"existingdir" help:"The project directory" default:"."`
	Spec      string            `type:"existingfile" help:"An optional specification file to copy into the project"`
	Variables map[string]string `arg:"" help:"Variables to pass to the template." optional:""`
}

func (c *InitCmd) Run(ctx *Context) error {
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

	projectDirInfo, projectDirErr := os.Stat(c.Dir)

	if c.fromNew {
		if projectDirErr == nil {
			return fmt.Errorf("%s already exists", c.Dir)
		}

		fmt.Printf("Creating project directory %s\n", c.Dir)
		if err = os.MkdirAll(c.Dir, 0777); err != nil {
			return err
		}
	} else {
		if projectDirErr != nil {
			return projectDirErr
		}
		if !projectDirInfo.IsDir() {
			return fmt.Errorf("%s is not a directory", c.Dir)
		}
	}

	if c.Variables == nil {
		c.Variables = map[string]string{}
	}
	// project name defaults to directory name
	if _, ok := c.Variables["name"]; !ok {
		name := filepath.Base(c.Dir)
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

	err = c.copy(templatePath, c.Dir, c.Variables)
	if err != nil {
		return err
	}

	if c.Spec != "" {
		if template.SpecLocation == "" {
			template.SpecLocation = "spec.apex"
		}

		specFilename := filepath.Join(c.Dir, filepath.Clean(template.SpecLocation))
		specBytes, err := os.ReadFile(c.Spec)
		if err != nil {
			return err
		}
		err = os.WriteFile(specFilename, specBytes, 0644)
		if err != nil {
			return err
		}
	}

	// TODO: Make dynamic (and secure)
	switch c.Template {
	case "@apexlang/codegen/local":
		cmd := exec.Command("npm", "install")
		cmd.Dir = filepath.Join(c.Dir, "codegen")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
	case "@apexlang/codegen/module":
		cmd := exec.Command("npm", "install")
		cmd.Dir = c.Dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (c *InitCmd) copy(source, destination string, variables map[string]string) error {
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
			return os.MkdirAll(dstPath, stat.Mode())
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
			if !c.fromNew {
				// If the file exists, skip writing it.
				if _, err := os.Stat(dstPath); err != nil {
					if !os.IsNotExist(err) {
						return err
					}
				} else {
					return nil // File exists so skip.
				}
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
