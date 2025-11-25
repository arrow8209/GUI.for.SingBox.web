package bridge

import (
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	sysruntime "runtime"

	"gopkg.in/yaml.v3"

	"guiforcores/pkg/eventbus"
)

var Config = &AppConfig{}

const (
	windowStateNormal    = 0
	windowStateMinimised = 2
)

var Env = &EnvResult{
	IsStartup:   true,
	FromTaskSch: false,
	AppName:     "",
	AppVersion:  "v1.15.1",
	BasePath:    "",
	OS:          sysruntime.GOOS,
	ARCH:        sysruntime.GOARCH,
}

// NewApp initialises environment/configuration and returns an App instance.
func NewApp(bus *eventbus.Bus) *App {
	if Env.BasePath == "" {
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		Env.BasePath = filepath.ToSlash(filepath.Dir(exePath))
		if !pathExists(filepath.Join(Env.BasePath, "data")) {
			if wd, err := os.Getwd(); err == nil && pathExists(filepath.Join(wd, "data")) {
				Env.BasePath = filepath.ToSlash(wd)
			}
		}
		ensureDir(filepath.Join(Env.BasePath, "data"))
		ensureDir(filepath.Join(Env.BasePath, "data", ".cache"))
		Env.AppName = filepath.Base(exePath)
		if slices.Contains(os.Args, "tasksch") {
			Env.FromTaskSch = true
		}
		loadConfig()
	}

	return &App{
		Bus: bus,
		Exit: func() {
			os.Exit(0)
		},
	}
}

func loadConfig() {
	b, err := os.ReadFile(filepath.Join(Env.BasePath, "data", "user.yaml"))
	if err == nil {
		_ = yaml.Unmarshal(b, &Config)
	}

	if Config.Width == 0 {
		Config.Width = 800
	}
	if Config.Height == 0 {
		Config.Height = 540
	}

	Config.StartHidden = Env.FromTaskSch && Config.WindowStartState == windowStateMinimised

	if !Env.FromTaskSch {
		Config.WindowStartState = windowStateNormal
	}
}

func (a *App) IsStartup() bool {
	if Env.IsStartup {
		Env.IsStartup = false
		return true
	}
	return false
}

func (a *App) RestartApp() FlagResult {
	exePath := filepath.ToSlash(filepath.Join(Env.BasePath, Env.AppName))

	cmd := exec.Command(exePath)
	SetCmdWindowHidden(cmd)

	if err := cmd.Start(); err != nil {
		return FlagResult{false, err.Error()}
	}

	return FlagResult{true, "Success"}
}

func (a *App) GetEnv() EnvResult {
	return EnvResult{
		AppName:    Env.AppName,
		AppVersion: Env.AppVersion,
		BasePath:   Env.BasePath,
		OS:         Env.OS,
		ARCH:       Env.ARCH,
	}
}

func (a *App) GetInterfaces() FlagResult {
	log.Printf("GetInterfaces")

	interfaces, err := net.Interfaces()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	var interfaceNames []string
	for _, inter := range interfaces {
		interfaceNames = append(interfaceNames, inter.Name)
	}

	return FlagResult{true, strings.Join(interfaceNames, "|")}
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func ensureDir(p string) {
	_ = os.MkdirAll(p, os.ModePerm)
}
