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

func (p *Profile) ExistName(name string) bool {
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

func NewManager(logWriter io.Writer) *Manager {
	return &Manager{
		log: log.New(logWriter, "", log.LstdFlags),
	}
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

func (m *Manager) LoadProfile() error {
	profPath := filepath.Join(minecraftPath, "launcher_profiles.json")

	m.InfoLog("Load profile:", profPath)
	profFile, err := os.Open(profPath)
	if err != nil {
		m.FatalLog("Failed open profile:", profPath)
		return err
	}
	defer profFile.Close()

	m.prof = &Profile{}
	if err = json.NewDecoder(profFile).Decode(m.prof); err != nil {
		m.FatalLog("Failed read profile:", profPath)
		return err
	}
	return nil
}

func (m *Manager) LoadRecipe(recipePath string) error {
	m.InfoLog("Load recipe:", recipePath)
	_, err := toml.DecodeFile(recipePath, m)
	if err != nil {
		m.FatalLog("Failed load recipe")
		return err
	}

	if m.Name != "" && !m.prof.ExistName(m.Name) {
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

func (m *Manager) DownloadMods() error {
	modsPath := filepath.Join(m.root, "mods")

	m.InfoLog("Start install mods to:", modsPath)
	if !existPath(modsPath) {
		m.InfoLog("Create mods directory")
		if err := os.MkdirAll(modsPath, 0755); err != nil {
			m.FatalLog("Failed create mods directory")
			return err
		}
	}

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
	return nil
}

func (m *Manager) DownloadResourcePacks() error {
	resourcepacksPath := filepath.Join(m.root, "resourcepacks")

	m.InfoLog("Start install resourcepacks to:", resourcepacksPath)
	if !existPath(resourcepacksPath) {
		m.InfoLog("Create resourcepacks directory")
		if err := os.MkdirAll(resourcepacksPath, 0755); err != nil {
			m.FatalLog("Failed create resourcepacks directory")
			return err
		}
	}

	for _, resourcepack := range m.ResourcePacks {
		resourcepackPath := filepath.Join(resourcepacksPath, resourcepack.Name)
		if existPath(resourcepackPath) {
			m.InfoLog("Already installed:", resourcepack.Name)
			continue
		}

		m.InfoLog("Start install:", resourcepack.Name)
		resourcepackFile, err := os.Create(resourcepackPath)
		if err != nil {
			m.ErrorLog(err, "Failed create file:", resourcepackPath)
			continue
		}

		m.InfoLog("Download from:", resourcepack.URL)
		remoteFile, err := http.Get(resourcepack.URL)
		if err != nil {
			m.ErrorLog(err, "Failed download:", resourcepack.URL)
			continue
		}
		defer remoteFile.Body.Close()

		m.InfoLog("Install to:", resourcepackPath)
		_, err = io.Copy(resourcepackFile, remoteFile.Body)
		if err != nil {
			m.ErrorLog(err, "Failed write to:", resourcepackPath)
			continue
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
	recipePath := flag.Arg(0)

	m := NewManager(os.Stdout)
	if err := m.Execute(recipePath); err != nil {
		fmt.Fprintln(os.Stderr, "mcm:", err)
		os.Exit(1)
	}
}
