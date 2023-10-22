package cdddru

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DockerConfig struct {
	DO_DOCKER_BUILD  bool     `json:"do_docker_build" yaml:"do_docker_build"`
	DOCKER_FILE      string   `json:"docker_file" yaml:"docker_file"`
	DOCKER_IMAGE     string   `json:"docker_image" yaml:"docker_image"`
	DOCKER_PLATFORMS []string `json:"docker_platforms" yaml:"docker_platforms"`
	DOCKER_SERVER    string   `json:"docker_server" yaml:"docker_server"`
	DOCKER_USER      string   `json:"docker_user" yaml:"docker_user"`
	DOCKER_TOKEN     string   `json:"docker_token" yaml:"docker_token"`
	parentLink       *Config
}

type dockerConfig struct {
	Auths map[string]string `json:"auths" yaml:"auths"`
}

func (dkrcfg *DockerConfig) SomeMethod(logger *Logger) (err error) {
	return
}

func (dkrcfg *DockerConfig) SetAuth(dockerConfigPath string) (err error) {

	if isExist, isDir, err := IsPathExists(filepath.Join(os.Getenv("HOME"), ".docker", "config.json")); isExist && !isDir {
		return err
	}

	// logger := dkrcfg.parentLink.logger
	if dockerConfigPath == "" {
		dockerConfigPath = "/run/configs/dockerconfig/"
	}
	if isExist, isDir, err := IsPathExists(filepath.Join(dockerConfigPath, "config.json")); isExist && !isDir {
		if err != nil {
			return err
		}
		// os.Setenv("DOCKER_CONFIG", dockerConfigPath)
		CopyFile(filepath.Join(dockerConfigPath, "config.json"), filepath.Join(os.Getenv("HOME"), ".docker", "config.json"))
		return nil
	}

	// sets auth file from env variables:
	// DOCKER_SERVER
	// DOCKER_TOKEN
	// DOCKER_USER
	dockerServer := os.Getenv("DOCKER_SERVER")
	dockerToken := os.Getenv("DOCKER_TOKEN")
	dockerUser := os.Getenv("DOCKER_USER")

	if len(dockerServer) == 0 || len(dockerToken) == 0 || len(dockerUser) == 0 {
		return fmt.Errorf("not enough values in variables for docker authentication")
	}
	authStr := dockerUser + ":" + dockerToken
	base64Auth := base64.StdEncoding.EncodeToString([]byte(authStr))

	authEntries := map[string]string{dockerServer: base64Auth}

	dataToWrite, err := PrettyJsonEncodeToString(dockerConfig{Auths: authEntries})

	if err != nil {
		return err
	}
	envConfigPath := "/run/configs/dockerconfig_env"
	os.MkdirAll(envConfigPath, 0700)
	err = os.WriteFile(filepath.Join(envConfigPath, "config.json"), []byte(strings.ToLower(dataToWrite)), 0600)
	if err != nil {
		return err
	}
	CopyFile(filepath.Join(dockerConfigPath, "config.json"), filepath.Join(os.Getenv("HOME"), ".docker", "config.json"))
	// os.Setenv("DOCKER_CONFIG", envConfigPath)
	return nil
}

func (dcrcfg *DockerConfig) DockerImageBuildx(imageNameAndTag, contextPath string, platforms []string, logger *Logger) error {

	err = os.Chdir(contextPath)
	if err != nil {
		return fmt.Errorf("docker build and push failed: %w", err)
	}
	var stdout string
	stdout, err = RunExternalCmd("", "buildx error:", "docker", "buildx", "build",
		"--push", "--platform", strings.Join(platforms, ","), "--progress=plain", "-t", imageNameAndTag, ".")
	PrintInfo(logger, "%s", stdout)

	return err
}
