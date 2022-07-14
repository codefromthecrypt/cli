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
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type WatchCmd struct {
	Configs []string `arg:"" help:"The code generation configuration files" type:"existingfile" optional:""`
}

type ConfigAndPath struct {
	Config
	Path string
}

func (c *WatchCmd) Run(ctx *Context) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}

	if len(c.Configs) == 0 {
		c.Configs = append(c.Configs, "apex.yaml")
	}

	specs := make(map[string][]ConfigAndPath)
	for _, config := range c.Configs {
		configPath := filepath.Dir(config)
		fileConfigs, err := readConfigs(config)
		if err != nil {
			return err
		}

		for _, config := range fileConfigs {
			specFile := filepath.Join(configPath, config.Spec)
			configs := specs[specFile]
			configs = append(configs, ConfigAndPath{config, configPath})
			specs[specFile] = configs
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("Modified spec:", event.Name)
					g := GenerateCmd{}
					configs := specs[event.Name]
					for _, config := range configs {
						if err := os.Chdir(config.Path); err != nil {
							log.Printf("Error running generate: %v", err)
							continue
						}

						if g.generateConfig(config.Config); err != nil {
							log.Printf("Error running generate: %v", err)
						}
					}

					os.Chdir(path)

					log.Println("Watching for file changes.")
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	for spec := range specs {
		log.Printf("Watching %s...", spec)
		if err = watcher.Add(spec); err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Watching for file changes.")
	<-done

	return nil
}
