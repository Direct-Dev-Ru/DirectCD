package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func startup() (Config, error) {
	var configPath string
	var config Config = Config{}

	usr, err := user.Current()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get current user: %v", err)
	}

	// determinate configPath
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	} else {
		executable, err := os.Executable()
		if err != nil {
			return Config{}, fmt.Errorf("failed to get executable: %v", err)
		}

		if executable == "go" {
			// Running as a script using 'go run'
			configPath = filepath.Join(usr.HomeDir, ".config", "ddru-cd-tool", "config.json")
			os.MkdirAll(configPath, os.ModePerm)
			if err != nil {
				return Config{}, fmt.Errorf("failed to create config path for current user: %v", err)
			}
		} else {
			// Running as a binary
			configPath = filepath.Join(filepath.Dir(executable), "config.json")
		}
	}

	// Read the config file
	configFileBytes, err := os.ReadFile(configPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to read config file: %v", err)
	} else if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig, nil
	}

	configFile := strings.ReplaceAll(string(configFileBytes), "{{$HOME}}", getEnvVar("HOME", "/root"))

	// Parse the config file
	err = json.Unmarshal([]byte(configFile), &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %v", err)
	}
	return config, nil
}
