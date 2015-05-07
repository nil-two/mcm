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
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

var minecraftPath string

func init() {
	switch runtime.GOOS {
	case "windows":
		path := os.Getenv("APPDATA")
		minecraftPath = filepath.Join(path, ".minecraft")
	default:
		path, err := homedir.Dir()
		if err != nil {
			panic(err)
		}
		minecraftPath = filepath.Join(path, ".minecraft")
	}
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
mcm: v0.4.0
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

type Package struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type Manager struct {
	log           *log.Logger
	prof          *Profile
	errors        []string
	root          string
	Name          string    `toml:"name"`
	Mods          []Package `toml:"mod"`
	ResourcePacks []Package `toml:"resourcepack"`
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
	fullPath, err := filepath.Abs(path)
	if err != nil {
		m.FatalLog("Invalid path:", err.Error())
	}

	m.InfoLog("Load recipe:", fullPath)
	_, err = toml.DecodeFile(fullPath, m)
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

func (m *Manager) Download(kind string, a []Package) error {
	rootPath := filepath.Join(m.root, kind)

	m.InfoLog("Start install "+kind+" to:", rootPath)
	if !exists(rootPath) {
		m.InfoLog("Create " + kind + " directory")
		if err := os.MkdirAll(rootPath, 0755); err != nil {
			m.FatalLog("Failed create " + kind + " directory")
			return err
		}
	}

	for _, p := range a {
		path := filepath.Join(rootPath, p.Name)
		if exists(path) {
			m.InfoLog("Already installed:", p.Name)
			continue
		}

		m.InfoLog("Start install:", p.Name)
		f, err := os.Create(path)
		if err != nil {
			m.ErrorLog(err, "Failed create file:", path)
			continue
		}

		m.InfoLog("Download from:", p.URL)
		res, err := http.Get(p.URL)
		if err != nil {
			m.ErrorLog(err, "Failed download:", p.URL)
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
	if err := m.Download("mods", m.Mods); err != nil {
		return err
	}
	if err := m.Download("resourcepacks", m.ResourcePacks); err != nil {
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
