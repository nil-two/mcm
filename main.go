package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

var minecraftPath string

func init() {
	homePath, err := homedir.Dir()
	if err != nil {
		panic(err)
	}
	minecraftPath = filepath.Join(homePath, ".minecraft")
}

func usage() {
	os.Stderr.WriteString(`
Usage: mcm [OPTION]... RECIPE

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
	log  *log.Logger
	Root string `toml:"root"`
	Mods []Mod  `toml:"mod"`
}

func NewManager(confPath string, logWriter io.Writer) (*Manager, error) {
	m := &Manager{
		log:  log.New(logWriter, "", log.LstdFlags),
		Root: minecraftPath,
	}

	_, err := toml.DecodeFile(confPath, &m)
	if err != nil {
		return nil, err
	}
	m.Root = filepath.Clean(m.Root)
	return m, nil
}

func (m *Manager) Download() error {
	var errors []string

	m.log.Println("INFO:", "Start mcm")
	for _, mod := range m.Mods {
		modPath := filepath.Join(m.Root, "mods", mod.Name)
		if isExist(modPath) {
			m.log.Println("INFO:", "Already installed:", mod.Name)
			continue
		}

		m.log.Println("INFO:", "Start install:", mod.Name)
		modFile, err := os.Create(modPath)
		if err != nil {
			m.log.Println("ERRO:", "Failed create file:", modPath)
			errors = append(errors, err.Error())
			continue
		}

		m.log.Println("INFO:", "Download from:", mod.URL)
		remoteFile, err := http.Get(mod.URL)
		if err != nil {
			m.log.Println("ERRO:", "Failed Download:", mod.Name)
			errors = append(errors, err.Error())
			continue
		}
		defer remoteFile.Body.Close()

		_, err = io.Copy(modFile, remoteFile.Body)
		if err != nil {
			m.log.Println("ERRO:", "Failed Write to:", modPath)
			errors = append(errors, err.Error())
			continue
		}
		m.log.Println("INFO:", "Complete install:", mod.Name)
	}
	m.log.Println("INFO:", "Finish mcm")

	if len(errors) > 0 {
		return fmt.Errorf("%d errors occurred:\n%s",
			len(errors), strings.Join(errors, "\n"))
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

	m, err := NewManager(flag.Arg(0), os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcm:", err)
		os.Exit(1)
	}
	if err = m.Download(); err != nil {
		fmt.Fprintln(os.Stderr, "mcm:", err)
		os.Exit(1)
	}
}
