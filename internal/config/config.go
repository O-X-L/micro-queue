package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/creasty/defaults"
	"gopkg.in/yaml.v3"
)

const VERSION = "1.0"
const REGEX_VALID_QUEUE_NAME = "^[-a-zA-Z0-9]{1,50}$"
const MIN_TOKEN_LEN = 20

type AppConfig struct {
	Settings SettingsConfig `yaml:"settings" required:"true"`
	Queues   []QueueConfig  `yaml:"queues" required:"true"`
	Tokens   []TokenConfig  `yaml:"tokens" required:"true"`
	TokenMap PermissionSetMap
}

type SettingsConfig struct {
	ListenAddr string `yaml:"listenAddr" default:"127.0.0.1"`
	ListenPort uint   `yaml:"listenPort" default:"10000"`
	Path       string `yaml:"path" default:"/tmp/micro_queue"`
}

type QueueConfig struct {
	Name string `yaml:"name" required:"true"`
	Kind string `yaml:"kind" default:"fifo"`
}

func (s *QueueConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(s); err != nil {
		return err
	}

	type plain QueueConfig
	if err := unmarshal((*plain)(s)); err != nil {
		return err
	}

	return nil
}

type TokenConfig struct {
	Name  string   `yaml:"name" required:"true"`
	Token string   `yaml:"token" required:"true"`
	Post  []string `yaml:"post"`
	Get   []string `yaml:"get"`
	Admin bool     `yaml:"admin" default:"false"`
}

func (s *TokenConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(s); err != nil {
		return err
	}

	type plain TokenConfig
	if err := unmarshal((*plain)(s)); err != nil {
		return err
	}

	return nil
}

type PermissionSet struct {
	Name    string
	CanPost map[string]bool
	CanGet  map[string]bool
	IsAdmin bool
}

type PermissionSetMap map[string]PermissionSet

var Global *AppConfig
var validQueueNameRegex = regexp.MustCompile(REGEX_VALID_QUEUE_NAME)
var MODE_DEV = os.Getenv("MODE_DEV") == "1"
var MODE_DEBUG = os.Getenv("MODE_DEBUG") == "1"

func LoadConfig(configPath string) (*AppConfig, error) {
	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file at %s: %w", configPath, err)
	}

	var cfg AppConfig
	if err := defaults.Set(&cfg); err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(yamlFile, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// validate
	if len(cfg.Queues) == 0 {
		return nil, fmt.Errorf("config error: 'queues' list cannot be empty")
	}
	if len(cfg.Tokens) == 0 {
		return nil, fmt.Errorf("config error: 'tokens' list cannot be empty")
	}
	for i, q := range cfg.Queues {
		if q.Name == "" {
			return nil, fmt.Errorf("config error: 'queue #%v' requires a name", i)
		}
		if q.Kind != "fifo" {
			return nil, fmt.Errorf("config error: 'queue #%v %v' is of unsupported kind", i, q.Name)
		}
		if !validQueueNameRegex.MatchString(q.Name) {
			return nil, fmt.Errorf(
				"config error: 'queue #%v' name can only contain letters,digits,hyphens and needs to be 1-50 characters",
				i,
			)
		}
	}
	for i, t := range cfg.Tokens {
		if t.Name == "" {
			return nil, fmt.Errorf("config error: 'token #%v' requires a name", i)
		}
		if len(t.Token) < MIN_TOKEN_LEN {
			return nil, fmt.Errorf("config error: 'token #%v %v' token should be at least %v characters", i, t.Name, MIN_TOKEN_LEN)

		}
	}

	// fast-lookup TokenMap
	cfg.TokenMap = make(PermissionSetMap)
	for _, tokenCfg := range cfg.Tokens {
		if tokenCfg.Token == "" {
			return nil, fmt.Errorf("config error: token for '%s' is empty", tokenCfg.Name)
		}
		if _, exists := cfg.TokenMap[tokenCfg.Token]; exists {
			return nil, fmt.Errorf("config error: token '%s' (for '%s') is duplicated", tokenCfg.Token, tokenCfg.Name)
		}

		// Create the permission set for fast map lookups
		permSet := PermissionSet{
			Name:    tokenCfg.Name,
			CanPost: make(map[string]bool),
			CanGet:  make(map[string]bool),
			IsAdmin: tokenCfg.Admin,
		}

		// *** UPDATED LOGIC ***
		// Only populate the granular permissions if the token is NOT an admin.
		// This respects the user's insight that the lists are irrelevant for admins.
		if !permSet.IsAdmin {
			for _, q := range tokenCfg.Post {
				permSet.CanPost[q] = true
			}
			for _, q := range tokenCfg.Get {
				permSet.CanGet[q] = true
			}
		}

		cfg.TokenMap[tokenCfg.Token] = permSet
	}

	Global = &cfg
	return Global, nil
}
