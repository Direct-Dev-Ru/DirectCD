package cdddru

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



func DockerImageBuild(dockerClient *client.Client, imageNameAndTag, contextPath, dockerfile, platform string, logger *Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1200)
	defer cancel()

	tar, err := archive.TarWithOptions(contextPath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	PrintInfo(logger, "%s", filepath.Join(contextPath, dockerfile))

	dockerfile_raw, err := os.ReadFile(filepath.Join(contextPath, dockerfile))
	if err != nil {
		return err
	}
	newdockerfile_raw := []byte(strings.ReplaceAll(string(dockerfile_raw), "$BUILDPLATFORM", platform))

	err = os.WriteFile(filepath.Join(contextPath, dockerfile+strings.ReplaceAll(platform, "/", "-")), newdockerfile_raw, 0755)
	if err != nil {
		return err
	}
	PrintInfo(logger, "%s", string(newdockerfile_raw))

	overallImageNameTag := fmt.Sprintf("%s_%s", imageNameAndTag, strings.ReplaceAll(platform, "/", "-"))
	opts := types.ImageBuildOptions{
		// Dockerfile: dockerfile,
		Dockerfile: dockerfile + strings.ReplaceAll(platform, "/", "-"),
		Tags:       []string{overallImageNameTag},
		Remove:     true,
		Platform:   platform,
	}
	res, err := dockerClient.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(contextPath, dockerfile+strings.ReplaceAll(platform, "/", "-")))
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
		logger.Debug(lastLineReplaced)
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

func DockerImagePush(dockerClient *client.Client, imageNameAndTag, platform string, cfg Config, logger *Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1200)
	defer cancel()

	var authConfig = registry.AuthConfig{
		Username:      cfg.DOCKER.DOCKER_USER,
		Password:      cfg.DOCKER.DOCKER_TOKEN,
		ServerAddress: cfg.DOCKER.DOCKER_SERVER,
	}

	if len(cfg.DOCKER.DOCKER_TOKEN) == 0 {
		pathToDockerConfig := filepath.Join(GetEnvVar("HOME", "/root"), ".docker", "config.json")
		dockerConfig, err := os.ReadFile(pathToDockerConfig)
		if err != nil {
			return err
		}

		dockerAuths := &DockerAuths{}
		json.Unmarshal(dockerConfig, dockerAuths)

		for server, auth := range dockerAuths.Auths {
			if strings.TrimSuffix(server, "/") == strings.TrimSuffix(cfg.DOCKER.DOCKER_SERVER, "/") {
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
	tag := fmt.Sprintf("%s_%s", imageNameAndTag, strings.ReplaceAll(platform, "/", "-"))

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
