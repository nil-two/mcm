package main

import (
	"encoding/json"
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
mcm: v0.3.0
`[1:])
}

func existPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type Profile struct {
	Profiles map[string]struct {
		GameDir       string
		LastVersionId string
	}
}

func LoadProfile() (*Profile, error) {
	profPath := filepath.Join(minecraftPath, "launcher_profiles.json")
	profFile, err := os.Open(profPath)
	if err != nil {
		return nil, err
	}
	defer profFile.Close()

	prof := Profile{}
	if err = json.NewDecoder(profFile).Decode(&prof); err != nil {
		return nil, err
	}
	return &prof, nil
}

func (p *Profile) ExistName(name string) bool {
	_, ok := p.Profiles[name]
	return ok
}

type Mod struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type Manager struct {
	log    *log.Logger
	errors []string
	root   string
	Name   string `toml:"name"`
	Mods   []Mod  `toml:"mod"`
}

func NewManager(confPath string, logWriter io.Writer) (*Manager, error) {
	m := &Manager{
		log: log.New(logWriter, "", log.LstdFlags),
	}
	_, err := toml.DecodeFile(confPath, &m)
	if err != nil {
		return nil, err
	}

	prof, err := LoadProfile()
	if err != nil {
		return nil, err
	}
	switch {
	case m.Name == "":
		m.root = minecraftPath
	case !prof.ExistName(m.Name):
		return nil, fmt.Errorf("invalid version name: %s", m.Name)
	case prof.Profiles[m.Name].GameDir == "":
		m.root = minecraftPath
	default:
		m.root = filepath.Join(m.root, prof.Profiles[m.Name].GameDir)
	}
	return m, nil
}

func (m *Manager) InfoLog(messages ...string) {
	m.log.Println("INFO:", strings.Join(messages, " "))
}

func (m *Manager) ErrorLog(err error, messages ...string) {
	m.log.Println("ERRO:", strings.Join(messages, " "))
	m.errors = append(m.errors, err.Error())
}

func (m *Manager) FatalLog(messages ...string) {
	m.log.Println("FATA:", strings.Join(messages, " "))
}

func (m *Manager) Download() error {
	modsPath := filepath.Join(m.root, "mods")

	m.InfoLog("Start mcm")
	if !existPath(modsPath) {
		m.InfoLog("Create mods directory")
		if err := os.MkdirAll(modsPath, 0755); err != nil {
			m.FatalLog("Failed create mods directory")
			return err
		}
	}

	m.InfoLog("Start install mods to:", modsPath)
	for _, mod := range m.Mods {
		modPath := filepath.Join(modsPath, mod.Name)
		if existPath(modPath) {
			m.InfoLog("Already installed:", mod.Name)
			continue
		}

		m.InfoLog("Start install:", mod.Name)
		modFile, err := os.Create(modPath)
		if err != nil {
			m.ErrorLog(err, "Failed create file:", modPath)
			continue
		}

		m.InfoLog("Download from:", mod.URL)
		remoteFile, err := http.Get(mod.URL)
		if err != nil {
			m.ErrorLog(err, "Failed download:", mod.URL)
			continue
		}
		defer remoteFile.Body.Close()

		m.InfoLog("Install to:", modPath)
		_, err = io.Copy(modFile, remoteFile.Body)
		if err != nil {
			m.ErrorLog(err, "Failed write to:", modPath)
			continue
		}
	}

	if len(m.errors) > 0 {
		return fmt.Errorf("%d errors occurred:\n%s",
			len(m.errors), strings.Join(m.errors, "\n"))
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
