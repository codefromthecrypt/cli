package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

var baseDependencies = map[string][]string{
	"@apexlang/core": {
		"src/@apexlang/core",
	},
}

func AddDependencies(dependencies map[string][]string) {
	for name, paths := range dependencies {
		baseDependencies[name] = paths
	}
}

func getHomeDirectory() (string, error) {
	homeDir, err := ensureHomeDirectory()
	if err != nil {
		return "", err
	}

	err = checkDependencies(homeDir, false)

	return homeDir, err
}

const tsconfigContents = `{
  "compilerOptions": {
    "module": "commonjs",
    "target": "esnext",
    "baseUrl": ".",
    "lib": [      
      "esnext"
    ],
    "outDir": "../dist"
  }
}
`

func ensureHomeDirectory() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	home, err = homedir.Expand(home)
	if err != nil {
		return "", err
	}

	homeDir := filepath.Join(home, ".apex")
	srcDir := filepath.Join(homeDir, "src")
	templatesDir := filepath.Join(homeDir, "templates")
	definitionsDir := filepath.Join(homeDir, "definitions")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if err = os.MkdirAll(srcDir, 0700); err != nil {
			return "", err
		}
	}

	// Create tsconfig.json inside the src directory for editing inside an IDE.
	tsconfigJSON := filepath.Join(srcDir, "tsconfig.json")
	if _, err := os.Stat(tsconfigJSON); os.IsNotExist(err) {
		if err = os.WriteFile(tsconfigJSON, []byte(tsconfigContents), 0644); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		if err = os.MkdirAll(templatesDir, 0700); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(definitionsDir); os.IsNotExist(err) {
		if err = os.MkdirAll(definitionsDir, 0700); err != nil {
			return "", err
		}
	}

	return homeDir, nil
}

func checkDependencies(homeDir string, forceDownload bool) error {
	missing := make(map[string]struct{}, len(baseDependencies))
	for dependency, checks := range baseDependencies {
		for _, check := range checks {
			check = strings.ReplaceAll(check, "/", string(filepath.Separator))
			if forceDownload {
				missing[dependency] = struct{}{}
			} else if _, err := os.Stat(filepath.Join(homeDir, check)); os.IsNotExist(err) {
				missing[dependency] = struct{}{}
			}
		}
	}

	if len(missing) > 0 {
		fmt.Println("Installing base dependencies...")
		for dependency := range missing {
			cmd := InstallCmd{
				Location: dependency,
			}
			if err := cmd.doRun(&Context{}, homeDir); err != nil {
				return err
			}
		}
	}

	return nil
}
