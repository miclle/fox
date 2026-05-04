package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	defaultConfigPath = "fox-openapi.yaml"
	defaultOutPath    = "api/openapi.yaml"
)

type Config struct {
	ConfigPath       string
	ConfigExplicit   bool
	Entry            string            `yaml:"entry"`
	Out              string            `yaml:"out"`
	Format           string            `yaml:"format"`
	Sources          []string          `yaml:"sources"`
	IncludeTestFiles bool              `yaml:"includeTestFiles"`
	Info             InfoConfig        `yaml:"info"`
	Servers          []ServerConfig    `yaml:"servers"`
	Tags             []TagConfig       `yaml:"tags"`
	SecuritySchemes  map[string]Scheme `yaml:"securitySchemes"`
	MetadataHook     string            `yaml:"metadataHook"`
	AutoAdd          bool              `yaml:"autoAdd"`
	Workdir          string            `yaml:"workdir"`
	KeepDriver       bool              `yaml:"keepDriver"`
	Verbose          bool              `yaml:"verbose"`
}

type InfoConfig struct {
	Title       string `yaml:"title"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

type ServerConfig struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

type ExternalDocsConfig struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

type TagConfig struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description"`
	ExternalDocs *ExternalDocsConfig `yaml:"externalDocs"`
}

type Scheme struct {
	Type             string      `yaml:"type"`
	Description      string      `yaml:"description"`
	Name             string      `yaml:"name"`
	In               string      `yaml:"in"`
	Scheme           string      `yaml:"scheme"`
	BearerFormat     string      `yaml:"bearerFormat"`
	Flows            *OAuthFlows `yaml:"flows"`
	OpenIDConnectURL string      `yaml:"openIdConnectUrl"`
}

type OAuthFlows struct {
	Implicit          *OAuthFlow `yaml:"implicit"`
	Password          *OAuthFlow `yaml:"password"`
	ClientCredentials *OAuthFlow `yaml:"clientCredentials"`
	AuthorizationCode *OAuthFlow `yaml:"authorizationCode"`
}

type OAuthFlow struct {
	AuthorizationURL string            `yaml:"authorizationUrl"`
	TokenURL         string            `yaml:"tokenUrl"`
	RefreshURL       string            `yaml:"refreshUrl"`
	Scopes           map[string]string `yaml:"scopes"`
}

type Overrides struct {
	ConfigPath          string
	ConfigExplicit      bool
	Entry               string
	EntrySet            bool
	Out                 string
	OutSet              bool
	Format              string
	FormatSet           bool
	Sources             []string
	SourcesSet          bool
	IncludeTestFiles    bool
	IncludeTestFilesSet bool
	MetadataHook        string
	MetadataHookSet     bool
	AutoAdd             bool
	AutoAddSet          bool
	Workdir             string
	WorkdirSet          bool
	KeepDriver          bool
	KeepDriverSet       bool
	Verbose             bool
	VerboseSet          bool
}

func LoadConfig(overrides Overrides) (Config, error) {
	cfg := Config{
		ConfigPath:      defaultConfigPath,
		Out:             defaultOutPath,
		Sources:         []string{"./..."},
		Info:            InfoConfig{Title: "Fox API", Version: "0.0.0"},
		SecuritySchemes: map[string]Scheme{},
		Workdir:         ".",
	}
	if overrides.ConfigPath != "" {
		cfg.ConfigPath = overrides.ConfigPath
	}
	if overrides.WorkdirSet {
		cfg.Workdir = overrides.Workdir
	}
	if !filepath.IsAbs(cfg.ConfigPath) && cfg.Workdir != "" {
		cfg.ConfigPath = filepath.Join(cfg.Workdir, cfg.ConfigPath)
	}
	cfg.ConfigExplicit = overrides.ConfigExplicit
	if err := loadConfigFile(&cfg); err != nil {
		return Config{}, err
	}
	applyOverrides(&cfg, overrides)
	if cfg.SecuritySchemes == nil {
		cfg.SecuritySchemes = map[string]Scheme{}
	}
	abs, err := filepath.Abs(cfg.Workdir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve workdir: %w", err)
	}
	cfg.Workdir = abs
	if cfg.Format == "" {
		cfg.Format = inferFormat(cfg.Out)
	}
	cfg.Format = strings.ToLower(cfg.Format)
	if cfg.Format != "yaml" && cfg.Format != "json" {
		return Config{}, fmt.Errorf("format must be yaml or json, got %q", cfg.Format)
	}
	if cfg.Entry == "" {
		return Config{}, errors.New("entry is required")
	}
	return cfg, nil
}

func loadConfigFile(cfg *Config) error {
	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !cfg.ConfigExplicit {
			return nil
		}
		return fmt.Errorf("read config %s: %w", cfg.ConfigPath, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config %s: %w", cfg.ConfigPath, err)
	}
	return nil
}

func applyOverrides(cfg *Config, o Overrides) {
	if o.EntrySet {
		cfg.Entry = o.Entry
	}
	if o.OutSet {
		cfg.Out = o.Out
	}
	if o.FormatSet {
		cfg.Format = o.Format
	}
	if o.SourcesSet {
		cfg.Sources = append([]string(nil), o.Sources...)
	}
	if o.IncludeTestFilesSet {
		cfg.IncludeTestFiles = o.IncludeTestFiles
	}
	if o.MetadataHookSet {
		cfg.MetadataHook = o.MetadataHook
	}
	if o.AutoAddSet {
		cfg.AutoAdd = o.AutoAdd
	}
	if o.WorkdirSet {
		cfg.Workdir = o.Workdir
	}
	if o.KeepDriverSet {
		cfg.KeepDriver = o.KeepDriver
	}
	if o.VerboseSet {
		cfg.Verbose = o.Verbose
	}
}

func inferFormat(out string) string {
	if strings.EqualFold(filepath.Ext(out), ".json") {
		return "json"
	}
	return "yaml"
}
