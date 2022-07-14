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
	"os"
	"path/filepath"
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
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, c.Dir)
	if err != nil {
		return err
	}

	initCmd := InitCmd{
		fromNew:   true,
		Dir:       projectPath,
		Template:  c.Template,
		Spec:      c.Spec,
		Variables: c.Variables,
	}

	return initCmd.Run(ctx)
}
