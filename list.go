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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"gopkg.in/yaml.v3"
)

type ListCmd struct {
	Templates ListTemplatesCmd `cmd:"templates" help:"Lists installed templates"`
}

type ListTemplatesCmd struct {
}

func (c *ListTemplatesCmd) Run(ctx *Context) error {
	homeDir, err := getHomeDirectory()
	if err != nil {
		return err
	}

	templatesPath := filepath.Join(homeDir, "templates")
	type template struct {
		name string
		file string
	}
	var templates []template

	if err = filepath.Walk(templatesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return nil
		}

		if !info.IsDir() && info.Name() == ".template" {
			relPath, err := filepath.Rel(templatesPath, filepath.Dir(path))
			if err != nil {
				return err
			}
			templateName := strings.ReplaceAll(relPath, string(filepath.Separator), "/")
			templates = append(templates, template{templateName, path})
		}

		return nil
	}); err != nil {
		return err
	}

	t := table.NewWriter()
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name:   "Name",
			Colors: text.Colors{text.FgGreen},
		},
		{
			Name:   "Description",
			Colors: text.Colors{text.FgCyan},
		},
	})
	t.AppendHeader(table.Row{"Name", "Description"})
	for _, tmpl := range templates {
		templateBytes, err := os.ReadFile(tmpl.file)
		if err != nil {
			return err
		}

		var template Template
		if err = yaml.Unmarshal(templateBytes, &template); err != nil {
			return err
		}

		t.AppendRow(table.Row{tmpl.name, template.Description})
	}
	fmt.Println(t.Render())

	return nil
}
