package cdddru

import (
	"fmt"
	"os"
	"strings"
)

type DockerConfig struct {
	DO_DOCKER_BUILD bool   `json:"do_docker_build,string,omitempty" yaml:"do_docker_build"`
	DOCKER_FILE     string `json:"docker_file" yaml:"docker_file"`
	DOCKER_IMAGE    string `json:"docker_image" yaml:"docker_image"`
	DOCKER_SERVER   string `json:"docker_server" yaml:"docker_server"`
	DOCKER_USER     string `json:"docker_user" yaml:"docker_user"`
	DOCKER_TOKEN    string `json:"docker_token" yaml:"docker_token"`
	parentLink      *Config
}

func (dkrcfg *DockerConfig) SomeMethod(logger *Logger) (err error) {
	return
}

func (dcrcfg *DockerConfig) DockerImageBuildx(imageNameAndTag, contextPath string, platforms []string, logger *Logger) error {

	err = os.Chdir(contextPath)
	if err != nil {
		return fmt.Errorf("docker build and push failed: %w", err)
	}
	var stdout string
	stdout, err = RunExternalCmd("", "buildx error:", "docker", "buildx", "build",
		"--push", "--platform", strings.Join(platforms, ","), "-t", imageNameAndTag, ".")
	PrintInfo(logger, "%s", stdout)

	return err
}
