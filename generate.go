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
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-sourcemap/sourcemap"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v3"
	"rogchap.com/v8go"

	"github.com/apexlang/cli/js"
)

type Context struct{}

type GenerateCmd struct {
	Config string `arg:"" help:"The code generation configuration file" type:"existingfile" optional:""`

	prettier *js.JS
	once     sync.Once
}

type Config struct {
	Spec      string                 `json:"spec" yaml:"spec"`
	Config    map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Generates map[string]Target      `json:"generates" yaml:"generates"`
}

type Target struct {
	Module       string                 `json:"module" yaml:"module"`
	VisitorClass string                 `json:"visitorClass" yaml:"visitorClass"`
	IfNotExists  bool                   `json:"ifNotExists,omitempty" yaml:"ifNotExists,omitempty"`
	Executable   bool                   `json:"executable,omitempty" yaml:"executable,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	RunAfter     []Command              `json:"runAfter" yaml:"runAfter"`
}

type Command struct {
	Command string `json:"command" yaml:"command"`
	Dir     string `json:"dir" yaml:"dir"`
}

const generateTemplate = `import { parse } from "@apexlang/core";
import { Context, Writer } from "@apexlang/core/model";
import {{importClass}} from "{{module}}";

function resolver(location, from) {
  const source = resolverCallback(location, from);
  if (source.startsWith("error: ")) {
    throw source.substring(7);
  }
  return source;
}

export function generate(spec, config) {
  const doc = parse(spec, resolver);
  const context = new Context(config, doc);

  const writer = new Writer();
  const visitor = new {{visitorClass}}(writer);
  context.accept(context, visitor);
  let source = writer.string();

  return source;
}

js_exports["generate"] = generate;`

type errorGroup interface {
	Errors() []error
}

func (c *GenerateCmd) Run(ctx *Context) error {
	defer func() {
		if c.prettier != nil {
			c.prettier.Dispose()
		}
	}()

	if c.Config == "" {
		c.Config = "apex.yaml"
	}

	configs, err := readConfigs(c.Config)
	if err != nil {
		return err
	}

	var merr error
	for _, config := range configs {
		if err := c.generate(config); err != nil {
			merr = multierr.Append(merr, err)
		}
	}

	if merr != nil {
		var errors []error
		group, ok := err.(errorGroup)
		if ok {
			errors = group.Errors()
		} else {
			errors = []error{merr}
		}
		if len(errors) == 1 {
			return errors[0]
		}

		return fmt.Errorf("generation failed due to %d error(s)", len(errors))
	}

	return nil
}

func (c *GenerateCmd) generateConfig(config Config) error {
	defer func() {
		if c.prettier != nil {
			c.prettier.Dispose()
		}
	}()

	return c.generate(config)
}

func (c *GenerateCmd) generate(config Config) error {
	specBytes, err := readFile(config.Spec)
	if err != nil {
		return err
	}
	spec := string(specBytes)

	homeDir, err := getHomeDirectory()
	if err != nil {
		return err
	}
	srcDir := filepath.Join(homeDir, "node_modules")

	var merr error

	for filename, target := range config.Generates {
		if target.Module == "" {
			merr = appendAndPrintError(merr, "module is required for %s", filename)
			continue
		}
		importClass := "{ " + target.VisitorClass + " }"
		visitorClass := target.VisitorClass
		if target.VisitorClass == "" {
			importClass = "DefaultVisitor"
			visitorClass = importClass
		}
		if target.IfNotExists {
			_, err := os.Stat(filename)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil {
				fmt.Printf("Skipping %s...\n", filename)
				continue
			}
		}

		// Merge global config into target config
		if target.Config == nil && config.Config != nil {
			target.Config = make(map[string]interface{}, len(config.Config))
		}
		for k, v := range config.Config {
			if _, exists := target.Config[k]; !exists {
				target.Config[k] = v
			}
		}

		fmt.Printf("Generating %s...\n", filename)
		generateTS := generateTemplate
		generateTS = strings.Replace(generateTS, "{{module}}", target.Module, 1)
		generateTS = strings.Replace(generateTS, "{{importClass}}", importClass, 1)
		generateTS = strings.Replace(generateTS, "{{visitorClass}}", visitorClass, 1)

		// Get working directory so that modules can be loaded
		// relative to the project's root directory.
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = "."
		}

		result := api.Build(api.BuildOptions{
			Stdin: &api.StdinOptions{
				Contents:   generateTS,
				Sourcefile: "generate.ts",
				ResolveDir: workingDir,
			},
			Outdir:        ".",
			Sourcemap:     api.SourceMapExternal,
			Bundle:        true,
			AbsWorkingDir: workingDir,
			NodePaths:     []string{workingDir, srcDir},
			LogLevel:      api.LogLevelWarning,
		})
		if len(result.Errors) > 0 {
			return fmt.Errorf("esbuild returned errors: %v", result.Errors)
		}
		if len(result.OutputFiles) != 2 {
			return errors.New("esbuild did not produce exactly 2 output files")
		}

		bundle := string(result.OutputFiles[1].Contents)
		smapBytes := result.OutputFiles[0].Contents
		smap, err := sourcemap.Parse(result.OutputFiles[1].Path, smapBytes)
		if err != nil {
			return errors.New("could not parse sourcemap")
		}

		definitionsDir := filepath.Join(homeDir, "definitions")

		resolverCallback := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			iso := info.Context().Isolate()

			if len(info.Args()) < 1 {
				value, _ := v8go.NewValue(iso, "error: resolve: invalid arguments")
				return value
			}

			location := info.Args()[0].String()

			loc := filepath.Join(definitionsDir, filepath.Join(strings.Split(location, "/")...))
			if filepath.Ext(loc) != ".apex" {
				specLoc := loc + ".apex"
				found := false
				stat, err := os.Stat(specLoc)
				if err == nil && !stat.IsDir() {
					found = true
					loc = specLoc
				}

				if !found {
					stat, err := os.Stat(loc)
					if err != nil {
						value, _ := v8go.NewValue(iso, fmt.Sprintf("error: %v", err))
						return value
					}
					if stat.IsDir() {
						loc = filepath.Join(loc, "index.apex")
					} else {
						loc += ".apex"
					}
				}
			}

			data, err := os.ReadFile(loc)
			if err != nil {
				value, _ := v8go.NewValue(iso, fmt.Sprintf("error: %v", err))
				return value
			}

			value, _ := v8go.NewValue(iso, string(data))
			return value
		}

		j, err := js.Compile(bundle, map[string]v8go.FunctionCallback{
			"resolverCallback": resolverCallback,
		})
		if err != nil {
			merr = appendAndPrintError(merr, "Compilation error: %w", err)
			continue
		}
		defer j.Dispose()

		configMap := make(map[string]interface{}, len(config.Config)+len(target.Config))
		for k, v := range config.Config {
			configMap[k] = v
		}
		for k, v := range target.Config {
			configMap[k] = v
		}
		configMap["$filename"] = filename
		res, err := j.Invoke("generate", spec, configMap)
		if err != nil {
			if jserr, ok := err.(*v8go.JSError); ok {
				stackTrace := translateStackTrace(smap, jserr.StackTrace)
				merr = appendAndPrintError(merr, "%s", stackTrace)
			} else {
				merr = appendAndPrintError(merr, "Generation error: %w", err)
			}
			continue
		}

		source := res.(string)
		ext := filepath.Ext(filename)
		switch ext {
		case ".ts":
			source, err = c.formatTypeScript(source)
			if err != nil {
				merr = appendAndPrintError(merr, "Error formatting TypeScript: %w", err)
				continue
			}
		case ".cs":
			source, err = Astyle(source, "indent-namespaces break-blocks pad-comma indent=tab style=1tbs")
			if err != nil {
				merr = appendAndPrintError(merr, "Error formatting C#: %w", err)
				continue
			}
		case ".java", "c", "cpp", "c++", "h", "hpp", "h++", "m":
			source, err = Astyle(source, "pad-oper indent=tab style=google")
			if err != nil {
				merr = appendAndPrintError(merr, "Error formatting Java/C/C++/Objective-C: %w", err)
				continue
			}
		}

		dir := filepath.Dir(filename)
		if dir != "" {
			if err = os.MkdirAll(dir, 0777); err != nil {
				merr = appendAndPrintError(merr, "Error creating directory: %w", err)
				continue
			}
		}

		fileMode := fs.FileMode(0666)
		if target.Executable {
			fileMode = 0777
		}
		if err = os.WriteFile(filename, []byte(source), fileMode); err != nil {
			merr = appendAndPrintError(merr, "Error writing file: %w", err)
			continue
		}
	}

	// Some CLI-based formatters actually check for types referenced in other files
	// so we must call these after all the files are generated.
	for filename := range config.Generates {
		ext := filepath.Ext(filename)
		switch ext {
		case ".rs":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatRust(filename); err != nil {
				merr = appendAndPrintError(merr, "Error formatting Rust: %w", err)
				continue
			}
		case ".go":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatGolang(filename); err != nil {
				merr = appendAndPrintError(merr, "Error formatting Go: %w", err)
				continue
			}
		case ".py":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatPython(filename); err != nil {
				merr = appendAndPrintError(merr, "Error formatting Python: %w", err)
				continue
			}
		}
	}

	for _, target := range config.Generates {
		for _, command := range target.RunAfter {
			lines := strings.Split(strings.TrimSpace(command.Command), "\n")
			for i := range lines {
				lines[i] = strings.TrimSpace(lines[i])
			}
			joined := strings.Join(lines, " ")
			commandParts := strings.Split(joined, " ")
			fmt.Println("Running:", joined)
			cmd := exec.Command(commandParts[0], commandParts[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Dir = command.Dir
			if err = cmd.Run(); err != nil {
				merr = appendAndPrintError(merr, "Error running command: %s, %w", joined, err)
				continue
			}
		}
	}

	return merr
}

//go:embed prettier.js
var prettierSource string

func (c *GenerateCmd) formatTypeScript(source string) (string, error) {
	var err error
	c.once.Do(func() {
		c.prettier, err = js.Compile(prettierSource)
	})
	if err != nil {
		return "", err
	}

	res, err := c.prettier.Invoke("formatTypeScript", source)
	if err != nil {
		return "", err
	}

	return res.(string), nil
}

func formatRust(filename string) error {
	cmd := exec.Command("rustfmt", "--edition", "2021", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatGolang(filename string) error {
	cmd := exec.Command("gofmt", "-w", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatPython(filename string) error {
	cmd := exec.Command("yapf", "-i", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readFile(file string) ([]byte, error) {
	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		resp, err := http.Get(file)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		return io.ReadAll(resp.Body)
	}

	return os.ReadFile(file)
}

func readConfigs(configFile string) ([]Config, error) {
	configBytes, err := readFile(configFile)
	if err != nil {
		return nil, err
	}

	configYAMLs := strings.Split(string(configBytes), "---")
	configs := make([]Config, len(configYAMLs))
	for i, configYAML := range configYAMLs {
		var config Config
		if err := yaml.Unmarshal([]byte(configYAML), &config); err != nil {
			return nil, err
		}
		if config.Spec == "" {
			return nil, errors.New("spec is required")
		}
		if len(config.Generates) == 0 {
			return nil, errors.New("generates is required")
		}
		configs[i] = config
	}

	return configs, nil
}

func appendAndPrintError(merr error, format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	fmt.Println(err)
	return multierr.Append(merr, err)
}

func translateStackTrace(smap *sourcemap.Consumer, stackTrace string) string {
	lines := strings.Split(stackTrace, "\n")
	for i := 1; i < len(lines); i++ {
		l := strings.TrimRight(lines[i], " \t")
		idx := strings.LastIndex(l, "(")
		bundleIdx := strings.LastIndex(l, "bundle.js:")

		if strings.HasSuffix(l, ")") && idx != -1 {
			loc := l[idx+1:]
			loc = loc[:len(loc)-1]
			l = l[:idx]
			if source, line, column, ok := translateLocation(smap, loc); ok {
				lines[i] = fmt.Sprintf("%s(%s:%d:%d)", l, source, line, column)
			}
		} else if bundleIdx != -1 {
			loc := l[bundleIdx+1:]
			l = l[:bundleIdx]
			if source, line, column, ok := translateLocation(smap, loc); ok {
				lines[i] = fmt.Sprintf("%s(%s:%d:%d)", l, source, line, column)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func translateLocation(smap *sourcemap.Consumer, location string) (string, int, int, bool) {
	parts := strings.Split(location, ":")
	if len(parts) != 3 {
		return "", 0, 0, false
	}
	line, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, false
	}
	column, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, false
	}

	source, _, l, c, ok := smap.Source(line, column)
	if !ok {
		return "", 0, 0, false
	}
	if src, err := filepath.Abs(source); err == nil {
		source = src
	}

	return source, l, c, true
}
