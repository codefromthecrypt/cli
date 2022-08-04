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

package main

import (
	"fmt"
	"runtime"

	"github.com/alecthomas/kong"

	"github.com/apexlang/cli"
)

var version = "edge"

var commands struct {
	// Install installs a module into the module directory.
	Install cli.InstallCmd `cmd:"" help:"Install a module."`
	// Generate generates code driven by a configuration file.
	Generate cli.GenerateCmd `cmd:"" help:"Generate code from a configuration file."`
	// Watch watches configuration files for changes and triggers generate.
	Watch cli.WatchCmd `cmd:"" help:"Watch configuration files for changes and trigger code generation."`
	// List lists installed modules.
	List cli.ListCmd `cmd:"" help:"Lists installed modules."`
	// New creates a new project from a template.
	New cli.NewCmd `cmd:"" help:"Creates a new project from a template."`
	// Init initializes an existing project directory from a template.
	Init cli.InitCmd `cmd:"" help:"Initializes an existing project directory from a template."`
	// Upgrade reinstalls the base module dependencies.
	Upgrade cli.UpgradeCmd `cmd:"" help:"Upgrades to the latest base modules dependencies."`
	// Version prints out the version of this program and runtime info.
	Version versionCmd `cmd:""`
}

func main() {
	cli.AddModuleAliases(map[string]string{
		"local":  "@apexlang/core/local",
		"module": "@apexlang/core/module",
	})
	cli.AddDependencies(map[string][]string{
		"@apexlang/codegen": {
			"src/@apexlang/codegen",
			"templates/@apexlang/codegen",
			"definitions/@apexlang",
		},
	})
	ctx := kong.Parse(&commands)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run(&cli.Context{})
	ctx.FatalIfErrorf(err)
}

type versionCmd struct{}

func (c *versionCmd) Run() error {
	fmt.Printf("apex version %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
	return nil
}
