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

package js

import (
	"encoding/json"
	"fmt"
	"strings"

	"rogchap.com/v8go"
)

type JS struct {
	iso *v8go.Isolate
	ctx *v8go.Context
}

func Compile(source string, globals ...map[string]v8go.FunctionCallback) (*JS, error) {
	iso := v8go.NewIsolate()
	global := v8go.NewObjectTemplate(iso)
	console := v8go.NewObjectTemplate(iso)
	log := v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		args := make([]interface{}, len(info.Args()))
		for i, a := range info.Args() {
			args[i] = a
		}
		fmt.Println(args...)
		return nil
	})
	console.Set("log", log)
	global.Set("println", log)
	for _, g := range globals {
		for name, callback := range g {
			funcTemp := v8go.NewFunctionTemplate(iso, callback)
			global.Set(name, funcTemp)
		}
	}
	ctx := v8go.NewContext(iso, global)
	consoleObject, err := console.NewInstance(ctx)
	if err != nil {
		return nil, err
	}
	ctx.Global().Set("console", consoleObject)
	_, err = ctx.RunScript(`var js_exports = {};`, "exports.js")
	if err != nil {
		return nil, err
	}
	_, err = ctx.RunScript(source, "bundle.js")
	if err != nil {
		return nil, err
	}

	return &JS{
		iso: iso,
		ctx: ctx,
	}, nil
}

func (js *JS) Dispose() {
	js.ctx.Close()
	js.iso.Dispose()
}

func (js *JS) Invoke(function string, args ...interface{}) (interface{}, error) {
	global := js.ctx.Global()
	var argList strings.Builder

	for i, arg := range args {
		argName := fmt.Sprintf("arg_%d", i)
		value, err := js.convertInterface(arg)
		if err != nil {
			return nil, err
		}
		global.Set(argName, value)
		if i > 0 {
			argList.WriteString(", ")
		}
		argList.WriteString(argName)
	}

	res, err := js.ctx.RunScript(`js_exports.`+function+`(`+argList.String()+`);`, function)
	if err != nil {
		return nil, err
	}

	if res.IsString() {
		return res.String(), nil
	} else if res.IsInt32() {
		return res.Int32(), nil
	}

	return res, err
}

func (js *JS) convertInterface(value interface{}) (*v8go.Value, error) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return v8go.JSONParse(js.ctx, string(jsonBytes))
}
