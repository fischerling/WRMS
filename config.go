package main

import (
	"io"
	"os"
	"path"

	"github.com/google/uuid"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"muhq.space/go/wrms/llog"
)

type Config struct {
	Port          int            `yaml:"port"`
	Backends      []string       `yaml:"backends"`
	Playlists     []string       `yaml:"playlists"`
	LocalMusicDir string         `yaml:"music-dir"`
	UploadDir     string         `yaml:"upload-dir"`
	LogLevel      string         `yaml:"loglevel"`
	MpvFlags      string         `yaml:"mpv_flags"`
	AdminPW       string         `yaml:"admin-password"`
	Admins        []uuid.UUID    `yaml:"admins"`
	Spotify       *SpotifyConfig `yaml:"spotify"`
	TimeBonus     float64        `yaml:"time-bonus"`
	HasUpload     bool
}

func defaultConfig() Config {
	c := Config{Port: 8080, UploadDir: "uploads", LogLevel: "Info"}
	return c
}

func (c Config) IsAdmin(id uuid.UUID) bool {
	return slices.Contains(c.Admins, id)
}

func findConfig() string {
	confDir := os.Getenv("XDG_CONFIG_DIR")
	if confDir == "" {
		confDir = os.Getenv("HOME")
	}

	paths := []string{"config.yml",
		path.Join(confDir, "wrms", "config.yml")}

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if !os.IsNotExist(err) && !fileInfo.IsDir() {
			return path
		}
	}

	return ""
}

func loadConfig(configPath string) Config {
	f, err := os.Open(configPath)
	if err != nil {
		llog.Fatal("Opening config %s failed: %v", configPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		llog.Fatal("Reading config %s failed: %v", configPath, err)
	}

	config := defaultConfig()
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		llog.Fatal("Failed to parse config %s: %v", configPath, err)
	}

	return config
}

func newConfig() Config {
	configPath := findConfig()
	if configPath != "" {
		return loadConfig(configPath)
	}

	return defaultConfig()
}
