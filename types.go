package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

type Config struct {
	TASK_NAME          string `json:"task_name"`
	GIT_REPO_URL       string `json:"git_repo_url"`
	GIT_PRIVATE_KEY    string `json:"git_private_key"`
	GIT_START_TAG      string `json:"git_start_tag"`
	GIT_MAX_TAG        string `json:"git_max_tag"`
	GIT_BRANCH         string `json:"git_branch"`
	GIT_TAG_PREFIX     string `json:"git_tag_prefix"`
	GIT_START_TAG_FILE string `json:"git_start_tag_file"`
	DOCKER_IMAGE       string `json:"docker_image"`
	LOCAL_GIT_FOLDER   string `json:"local_git_folder"`
	// this subfolder from LOCAL_GIT_FOLDER will be rsynced into TARGET_FOLDER
	GIT_SUB_FOLDER string `json:"git_sub_folder"`
	// in this place subfolder from LOCAL_GIT_FOLDER will be rsynced
	TARGET_FOLDER       string `json:"target_folder"`
	CHECK_INTERVAL      int    `json:"check_interval"`
	MANIFESTS_K8S       string `json:"manifests_k8s"`
	DEPLOYMENT_NAME_K8s string `json:"deployment_name_k8s"`
	NAMESPACE_K8s       string `json:"namespace_k8s"`
}

var USER *user.User
var err error
var checkInterval int
var DefaultConfig Config

func init() {

	USER, err = user.Current()
	if err != nil {
		fmt.Printf("failed to get current user: %v\n", err)
		os.Exit(1)
	}
	checkInterval, err = strconv.Atoi(getEnvVar("CHECK_INTERVAL", "300"))
	if err != nil {
		checkInterval = 300
	}

	DefaultConfig = Config{
		TASK_NAME:           getEnvVar("TASK_NAME", "default deploy task"),
		GIT_REPO_URL:        getEnvVar("GIT_REPO_URL", "git@github.com:Direct-Dev-Ru/http2-nodejs-ddru.git"),
		GIT_PRIVATE_KEY:     getEnvVar("GIT_PRIVATE_KEY", filepath.Join(USER.HomeDir, ".ssh", "id_rsa")),
		GIT_START_TAG:       getEnvVar("GIT_START_TAG", "v1.0.0"),
		GIT_MAX_TAG:         getEnvVar("GIT_MAX_TAG", ""),
		GIT_BRANCH:          getEnvVar("GIT_BRANCH", "main"),
		GIT_TAG_PREFIX:      getEnvVar("GIT_TAG_PREFIX", "v"),
		GIT_START_TAG_FILE:  getEnvVar("GIT_START_TAG_FILE", "/usr/local/cdddru/start-tag"),
		DOCKER_IMAGE:        getEnvVar("DOCKER_IMAGE", "docker.io/kuznetcovay/ddru"),
		LOCAL_GIT_FOLDER:    getEnvVar("LOCAL_GIT_FOLDER", "/tmp/git_local_repo"),
		GIT_SUB_FOLDER:      getEnvVar("GIT_SUB_FOLDER", ""),                                //if empty - all repo to rsync
		TARGET_FOLDER:       getEnvVar("TARGET_FOLDER", filepath.Join(USER.HomeDir, "app")), //where web app is
		CHECK_INTERVAL:      checkInterval,
		MANIFESTS_K8S:       getEnvVar("MANIFESTS_K8S", filepath.Join(USER.HomeDir, "app", "k8s_deployment.yaml")),
		DEPLOYMENT_NAME_K8s: getEnvVar("DEPLOYMENT_NAME_K8S", "main-site"),
		NAMESPACE_K8s:       getEnvVar("NAMESPACE_K8S", "test-app"),
	}

}
