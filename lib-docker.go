package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

func dockerImageBuild(dockerClient *client.Client, imageNameAndTag, contextPath, dockerfile string, logger *Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1200)
	defer cancel()

	tar, err := archive.TarWithOptions(contextPath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := types.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{imageNameAndTag},
		Remove:     true,
	}
	res, err := dockerClient.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	err = PrintDockerResponse(res.Body, logger)
	if err != nil {
		return err
	}

	return nil
}

func PrintDockerResponse(rd io.Reader, logger *Logger) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()

		lastLineReplaced := strings.ReplaceAll(lastLine, `\n`, ``)
		lastLineReplaced, _ = strconv.Unquote(`"` + strings.ReplaceAll(lastLineReplaced, `"`, `\"`) + `"`)

		if err != nil {
			fmt.Println("Error:", err)
			return err
		}
		// fmt.Println("replaced:", lastLineReplaced)
		logger.Info(lastLineReplaced)
		// fmt.Println("original:", lastLineReplaced)
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func dockerImagePush(dockerClient *client.Client, imageNameAndTag string, cfg Config, logger *Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1200)
	defer cancel()

	var authConfig = registry.AuthConfig{
		Username:      cfg.DOCKER_USER,
		Password:      cfg.DOCKER_TOKEN,
		ServerAddress: cfg.DOCKER_SERVER,
	}

	if len(cfg.DOCKER_TOKEN) == 0 {
		pathToDockerConfig := filepath.Join(getEnvVar("HOME", "/root"), ".docker", "config.json")
		dockerConfig, err := os.ReadFile(pathToDockerConfig)
		if err != nil {
			return err
		}

		dockerAuths := &DockerAuths{}
		json.Unmarshal(dockerConfig, dockerAuths)

		for server, auth := range dockerAuths.Auths {
			if strings.TrimSuffix(server, "/") == strings.TrimSuffix(cfg.DOCKER_SERVER, "/") {
				innerAuth := auth.Auth
				decodedAuth, err := base64.StdEncoding.DecodeString(innerAuth)
				if err != nil {
					return err
				}
				authData := strings.Split(string(decodedAuth), ":")
				authConfig.ServerAddress = server
				authConfig.Auth = innerAuth
				authConfig.Username = authData[0]
				authConfig.Password = authData[1]

			}
		}
	}

	authConfigBytes, _ := json.Marshal(authConfig)
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)

	tag := imageNameAndTag
	opts := types.ImagePushOptions{RegistryAuth: authConfigEncoded}
	rd, err := dockerClient.ImagePush(ctx, tag, opts)
	if err != nil {
		return err
	}

	defer rd.Close()

	err = PrintDockerResponse(rd, logger)
	if err != nil {
		return err
	}

	return nil
}
