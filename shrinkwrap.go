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

type Shrinkwrap struct {
	Name            string             `json:"name"`
	Version         string             `json:"version"`
	LockfileVersion int                `json:"lockfileVersion"`
	Requires        bool               `json:"requires"`
	Packages        map[string]Package `json:"packages"`
	Dependencies    map[string]Package `json:"dependencies"`
}

type Package struct {
	Version    string `json:"version"`
	Resolved   string `json:"resolved"`
	Integrity  string `json:"integrity"`
	Dev        bool   `json:"dev"`
	Extraneous bool   `json:"extraneous"`
}
