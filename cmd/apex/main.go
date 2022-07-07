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
		"local":  "@apexlang/codegen/local",
		"module": "@apexlang/codegen/module",
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
