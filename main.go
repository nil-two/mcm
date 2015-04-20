package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

var homePath string

func init() {
	h, err := homedir.Dir()
	if err != nil {
		panic(err)
	}
	homePath = h
}

func usage() {
	os.Stderr.WriteString(`
Usage: mcm [OPTION]... [FILE]...

Options:
	--help       show this help message
	--version    print the version
`[1:])
}

func version() {
	os.Stderr.WriteString(`
mcm: v0.1.0
`[1:])
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

type Mod struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type Manager struct {
	Root string `toml:"root"`
	Mods []Mod  `toml:"mod"`
}

func NewManager(confPath string) (*Manager, error) {
	m := &Manager{
		Root: filepath.Join(homePath, ".minecraft"),
	}

	_, err := toml.DecodeFile(confPath, &m)
	if err != nil {
		return nil, err
	}
	m.Root = filepath.Clean(m.Root)
	return m, nil
}

func (m *Manager) Download() error {
	for _, mod := range m.Mods {
		modPath := filepath.Join(m.Root, "mods", mod.Name)
		if isExist(modPath) {
			continue
		}

		modFile, err := os.Create(modPath)
		if err != nil {
			return err
		}

		remoteFile, err := http.Get(mod.URL)
		if err != nil {
			return err
		}
		defer remoteFile.Body.Close()

		_, err = io.Copy(modFile, remoteFile.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	isHelp := flag.Bool("help", false, "")
	isVersion := flag.Bool("version", false, "")
	flag.Usage = usage
	flag.Parse()

	if *isHelp {
		usage()
		os.Exit(2)
	}
	if *isVersion {
		version()
		os.Exit(2)
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "mcm:", "no input file")
		usage()
		os.Exit(1)
	}

	m, err := NewManager(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcm:", err)
		os.Exit(1)
	}
	if err = m.Download(); err != nil {
		fmt.Fprintln(os.Stderr, "mcm:", err)
		os.Exit(1)
	}
}
