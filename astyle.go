package cli

import (
	"context"
	_ "embed"
	"errors"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed astyle.wasm
var astyleWasm []byte

func Astyle(source, options string) (string, error) {
	ctx := context.Background()
	rc := wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV2)
	r := wazero.NewRuntimeWithConfig(ctx, rc)
	config := wazero.NewModuleConfig().
		WithStartFunctions("_initialize").
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithSysWalltime().
		WithSysNanotime()

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return "", err
	}

	compiled, err := r.CompileModule(ctx, astyleWasm)
	if err != nil {
		return "", err
	}
	defer compiled.Close(ctx)

	module, err := r.InstantiateModule(ctx, compiled, config.WithName("astyle"))
	if err != nil {
		return "", err
	}
	defer module.Close(ctx)

	alloc := module.ExportedFunction("alloc_buffer")
	free := module.ExportedFunction("free_buffer")
	format := module.ExportedFunction("wastyle")
	if alloc == nil || free == nil || format == nil {
		return "", errors.New("missing exported function alloc_buffer, free_buffer, or wastyle")
	}

	sourceUTF8 := []byte(source)
	optionsUTF8 := []byte(options)
	bufferSize := uint32(len(sourceUTF8) + 1 + len(optionsUTF8) + 1 + 4)
	res, err := alloc.Call(ctx, uint64(bufferSize))
	if err != nil {
		return "", err
	}
	bufferPointer := uint32(res[0])

	mem := module.Memory()

	resultPointer := bufferPointer
	sourcePointer := resultPointer + 4
	optionsPointer := sourcePointer + uint32(len(sourceUTF8)) + 1

	mem.Write(ctx, sourcePointer, sourceUTF8)
	mem.WriteByte(ctx, sourcePointer+uint32(len(sourceUTF8)), 0)
	mem.Write(ctx, optionsPointer, optionsUTF8)
	mem.WriteByte(ctx, optionsPointer+uint32(len(optionsUTF8)), 0)

	result, err := format.Call(ctx,
		uint64(sourcePointer), uint64(optionsPointer), uint64(resultPointer))
	if err != nil {
		return "", err
	}
	success := result[0] == 1

	formattedPointer, ok := mem.ReadUint32Le(ctx, resultPointer)
	if !ok {
		return "", errors.New("could not read result pointer")
	}

	resultBuf, ok := mem.Read(ctx, formattedPointer, mem.Size(ctx)-formattedPointer)
	if !ok {
		return "", errors.New("could not read formatted source")
	}

	i := uint32(0)
	for resultBuf[i] != 0 {
		i++
	}
	formattedBytes := resultBuf[0:i]
	formattedSource := string(formattedBytes)

	free.Call(ctx, uint64(bufferPointer))
	if formattedPointer != 0 {
		free.Call(ctx, uint64(formattedPointer))
	}

	if !success {
		return "", errors.New(formattedSource)
	}

	return formattedSource, err
}
