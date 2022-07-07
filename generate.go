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
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
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
import { {{visitorClass}} } from "{{module}}";

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

	for _, config := range configs {
		if err := c.generate(config); err != nil {
			return err
		}
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
	srcDir := filepath.Join(homeDir, "src")

	for filename, target := range config.Generates {
		if target.Module == "" {
			return fmt.Errorf("module is required for %s", filename)
		}
		if target.VisitorClass == "" {
			return fmt.Errorf("visitorClass is required for %s", filename)
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
		generateTS = strings.Replace(generateTS, "{{visitorClass}}", target.VisitorClass, -1)

		result := api.Build(api.BuildOptions{
			Stdin: &api.StdinOptions{
				Contents:   generateTS,
				Sourcefile: "generate.ts",
				ResolveDir: srcDir,
			},
			Bundle:    true,
			NodePaths: []string{".", srcDir},
			LogLevel:  api.LogLevelInfo,
		})
		if len(result.Errors) > 0 {
			return fmt.Errorf("esbuild returned errors: %v", result.Errors)
		}
		if len(result.OutputFiles) != 1 {
			return errors.New("esbuild did not produce exactly 1 output file")
		}

		bundle := string(result.OutputFiles[0].Contents)

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
			return err
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
				jserr.Message = strings.TrimPrefix(jserr.Message, "Error: ")
			}
			return err
		}

		source := res.(string)
		ext := filepath.Ext(filename)
		switch ext {
		case ".ts":
			source, err = c.formatTypeScript(source)
			if err != nil {
				return err
			}
		}

		dir := filepath.Dir(filename)
		if dir != "" {
			if err = os.MkdirAll(dir, 0777); err != nil {
				return err
			}
		}

		fileMode := fs.FileMode(0666)
		if target.Executable {
			fileMode = 0777
		}
		if err = os.WriteFile(filename, []byte(source), fileMode); err != nil {
			return err
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
				return err
			}
		case ".go":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatGolang(filename); err != nil {
				return err
			}
		case ".py":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatPython(filename); err != nil {
				return err
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
				return err
			}
		}
	}

	return nil
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
	cmd := exec.Command("rustfmt", filename)
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
