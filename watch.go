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
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type WatchCmd struct {
	Configs []string `arg:"" help:"The code generation configuration files" type:"existingfile" optional:""`
}

func (c *WatchCmd) Run(ctx *Context) error {
	if len(c.Configs) == 0 {
		c.Configs = append(c.Configs, "apex.yaml")
	}
	for i, config := range c.Configs {
		config, err := filepath.Abs(config)
		if err != nil {
			return err
		}
		c.Configs[i] = config
	}

	configs := make(map[string][]string)
	specs := make(map[string][]Config)

	reloadConfigs := func() error {
		configs = make(map[string][]string)
		specs = make(map[string][]Config)

		for _, config := range c.Configs {
			fileConfigs, err := readConfigs(config)
			if err != nil {
				return err
			}

			configSpecs := []string{}
			for _, config := range fileConfigs {
				specFile, err := filepath.Abs(config.Spec)
				if err != nil {
					return err
				}
				configSpecs = append(configSpecs, specFile)
				configs := specs[specFile]
				configs = append(configs, config)
				specs[specFile] = configs
			}
			configs[config] = configSpecs
		}

		return nil
	}

	configWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer configWatcher.Close()

	specWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer specWatcher.Close()

	syncWatchers := func() error {
		currentSpecs := make(map[string]struct{})
		removeSpecs := make(map[string]struct{})
		for _, name := range specWatcher.WatchList() {
			currentSpecs[name] = struct{}{}
			removeSpecs[name] = struct{}{}
		}
		for _, specs := range configs {
			for _, spec := range specs {
				if _, exists := currentSpecs[spec]; exists {
					delete(removeSpecs, spec)
					continue
				}
				log.Printf("Watching %s...", spec)
				if err = specWatcher.Add(spec); err != nil {
					return err
				}
				currentSpecs[spec] = struct{}{}
			}
		}
		for name := range removeSpecs {
			log.Printf("Unwatching %s...", name)
			specWatcher.Remove(name)
		}

		return nil
	}

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-configWatcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write != fsnotify.Write {
					continue
				}

				log.Println("Modified config:", event.Name)
				if err := reloadConfigs(); err != nil {
					log.Println("error:", err)
					return
				}
				if err := syncWatchers(); err != nil {
					log.Println("error:", err)
					return
				}

				g := GenerateCmd{}
				if eventSpecs, ok := configs[event.Name]; ok {
					for _, eventSpec := range eventSpecs {
						configs := specs[eventSpec]
						for _, config := range configs {
							if g.generateConfig(config); err != nil {
								log.Printf("Error running generate: %v", err)
							}
						}
					}
				}

			case event, ok := <-specWatcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write != fsnotify.Write {
					continue
				}

				log.Println("Modified spec:", event.Name)
				g := GenerateCmd{}
				configs := specs[event.Name]
				for _, config := range configs {
					if g.generateConfig(config); err != nil {
						log.Printf("Error running generate: %v", err)
					}
				}

				log.Println("Watching for file changes.")

			case err, ok := <-configWatcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)

			case err, ok := <-specWatcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	reloadConfigs()
	for config := range configs {
		log.Printf("Watching %s...", config)
		if err = configWatcher.Add(config); err != nil {
			log.Fatal(err)
		}
	}
	syncWatchers()

	log.Println("Watching for file changes.")
	<-done

	return nil
}
