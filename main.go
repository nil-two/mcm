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

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type Profile struct {
	Profiles map[string]struct {
		GameDir       string
		LastVersionId string
	}
}

func (p *Profile) Valid(name string) bool {
	_, ok := p.Profiles[name]
	return ok
}

type Mod struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type ResourcePack struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type Manager struct {
	log           *log.Logger
	prof          *Profile
	errors        []string
	root          string
	Name          string         `toml:"name"`
	Mods          []Mod          `toml:"mod"`
	ResourcePacks []ResourcePack `toml:"resourcepack"`
}

func NewManager(w io.Writer) *Manager {
	return &Manager{
		log: log.New(w, "", log.LstdFlags),
	}
}

func (m *Manager) InfoLog(a ...string) {
	m.log.Println("INFO:", strings.Join(a, " "))
}

func (m *Manager) ErrorLog(err error, a ...string) {
	m.log.Println("ERRO:", strings.Join(a, " "))
	m.errors = append(m.errors, err.Error())
}

func (m *Manager) FatalLog(a ...string) {
	m.log.Println("FATA:", strings.Join(a, " "))
}

func (m *Manager) LoadProfile() error {
	path := filepath.Join(minecraftPath, "launcher_profiles.json")

	m.InfoLog("Load profile:", path)
	f, err := os.Open(path)
	if err != nil {
		m.FatalLog("Failed open profile:", path)
		return err
	}
	defer f.Close()

	m.prof = &Profile{}
	if err = json.NewDecoder(f).Decode(m.prof); err != nil {
		m.FatalLog("Failed read profile:", path)
		return err
	}
	return nil
}

func (m *Manager) LoadRecipe(path string) error {
	m.InfoLog("Load recipe:", path)
	_, err := toml.DecodeFile(path, m)
	if err != nil {
		m.FatalLog("Failed load recipe")
		return err
	}

	if m.Name != "" && !m.prof.Valid(m.Name) {
		m.FatalLog("Failed find the version name:", m.Name)
		return fmt.Errorf("invalid version name: %s", m.Name)
	}
	if m.Name == "" || m.prof.Profiles[m.Name].GameDir == "" {
		m.root = minecraftPath
		return nil
	}
	m.root = m.prof.Profiles[m.Name].GameDir
	return nil
}

func (m *Manager) DownloadMods() error {
	rootPath := filepath.Join(m.root, "mods")

	m.InfoLog("Start install mods to:", rootPath)
	if !exists(rootPath) {
		m.InfoLog("Create mods directory")
		if err := os.MkdirAll(rootPath, 0755); err != nil {
			m.FatalLog("Failed create mods directory")
			return err
		}
	}

	for _, mod := range m.Mods {
		path := filepath.Join(rootPath, mod.Name)
		if exists(path) {
			m.InfoLog("Already installed:", mod.Name)
			continue
		}

		m.InfoLog("Start install:", mod.Name)
		f, err := os.Create(path)
		if err != nil {
			m.ErrorLog(err, "Failed create file:", path)
			continue
		}

		m.InfoLog("Download from:", mod.URL)
		res, err := http.Get(mod.URL)
		if err != nil {
			m.ErrorLog(err, "Failed download:", mod.URL)
			continue
		}
		defer res.Body.Close()

		m.InfoLog("Install to:", path)
		_, err = io.Copy(f, res.Body)
		if err != nil {
			m.ErrorLog(err, "Failed write to:", path)
			continue
		}
	}
	return nil
}

func (m *Manager) DownloadResourcePacks() error {
	fullPath := filepath.Join(m.root, "resourcepacks")

	m.InfoLog("Start install resourcepacks to:", fullPath)
	if !exists(fullPath) {
		m.InfoLog("Create resourcepacks directory")
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			m.FatalLog("Failed create resourcepacks directory")
			return err
		}
	}

	for _, resourcepack := range m.ResourcePacks {
		path := filepath.Join(fullPath, resourcepack.Name)
		if exists(path) {
			m.InfoLog("Already installed:", resourcepack.Name)
			continue
		}

		m.InfoLog("Start install:", resourcepack.Name)
		f, err := os.Create(path)
		if err != nil {
			m.ErrorLog(err, "Failed create file:", path)
			continue
		}

		m.InfoLog("Download from:", resourcepack.URL)
		res, err := http.Get(resourcepack.URL)
		if err != nil {
			m.ErrorLog(err, "Failed download:", resourcepack.URL)
			continue
		}
		defer res.Body.Close()

		m.InfoLog("Install to:", path)
		_, err = io.Copy(f, res.Body)
		if err != nil {
			m.ErrorLog(err, "Failed write to:", path)
			continue
		}
	}
	return nil
}

func (m *Manager) Execute(recipePath string) error {
	m.InfoLog("Start mcm")
	if err := m.LoadProfile(); err != nil {
		return err
	}
	if err := m.LoadRecipe(recipePath); err != nil {
		return err
	}
	if err := m.DownloadMods(); err != nil {
		return err
	}
	if err := m.DownloadResourcePacks(); err != nil {
		return err
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
	recipe := flag.Arg(0)

	m := NewManager(os.Stdout)
	if err := m.Execute(recipe); err != nil {
		fmt.Fprintln(os.Stderr, "mcm:", err)
		os.Exit(1)
	}
}
