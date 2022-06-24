# Apex

**noun**

__1__. the top or highest part of something

**language**

__2__. a top-down / API-first description language for modeling and generating cloud-native applications

### Goals:

* <ins>A</ins>pproachable
* <ins>P</ins>rotocol agnostic
* <ins>Ex</ins>tensible
 
### Problem
 
TODO
 
### Solution
 
TODO

### Overview

TODO

## Installation

Windows

```
powershell -Command "iwr -useb https://raw.githubusercontent.com/apexlang/cli/main/install/install.ps1 | iex"
```

MacOS

```
curl -fsSL https://raw.githubusercontent.com/apexlang/cli/main/install/install.sh | /bin/bash
```

Linux

```
wget -q https://raw.githubusercontent.com/apexlang/cli/main/install/install.sh -O - | /bin/bash
```

Homebrew

```
brew install apexlang/tap/apex
```

## Building a Module

TODO

## Development

### Prerequisites

Verify you have Go 1.17+ installed

```shell
go version
```

If Go is not installed, [download and install Go 1.17+](https://golang.org/dl/) (brew installation is not recommended because of CGO linker warnings)

Clone the project from github

```shell
git clone https://github.com/apexlang/apex.git
cd cli
go install ./cmd/...
```

**Compiling on Windows**

In order to build a project using v8go on Windows, Go requires a gcc compiler to be installed.

To set this up:
1. Install MSYS2 (https://www.msys2.org/)
2. Add the Mingw-w64 bin to your PATH environment variable (`C:\msys64\mingw64\bin` by default)
3. Open MSYS2 MSYS and execute `pacman -S mingw-w64-x86_64-toolchain`

V8 requires 64-bit on Windows, therefore it will not work on 32-bit systems. 

Confirm `apex` runs (The Go installation should add `~/go/bin` in your `PATH`)

```shell
apex --help
```

Output:

```
Usage: apex <command>

Flags:
  -h, --help    Show context-sensitive help.

Commands:
  install <location> [<release>]
    Install a module.

  generate [<config>]
    Generate code from a configuration file.

  watch [<configs> ...]
    Watch configuration files for changes and trigger code generation.

  new <template> <dir> [<variables> ...]
    Creates a new project from a template.

  upgrade
    Upgrades to the latest base modules dependencies.

  version

Run "apex <command> --help" for more information on a command.
```

## Built With

* [esbuild](https://esbuild.github.io/) - An extremely fast JavaScript bundler written in Go that is used to compile the code generation TypeScript modules into JavaScript that can run natively in V8.
* [v8go](https://github.com/rogchap/v8go) and [V8](https://v8.dev/) - Execute JavaScript from Go
* [kong](https://github.com/alecthomas/kong) - A very simple and easy to use command-line parser for Go
* [The Go 1.16 embed package](https://golang.org/pkg/embed/) - Finally embedding files is built into the Go toolchain!

## Contributing

Please read [CONTRIBUTING.md](https://github.com/apexlang/cli/blob/main/CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/apexlang/cli/tags).

## License

This project is licensed under the [Apache License 2.0](https://choosealicense.com/licenses/apache-2.0/) - see the [LICENSE.txt](LICENSE.txt) file for details
