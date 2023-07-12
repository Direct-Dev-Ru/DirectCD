package cdddru

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	JOB_NAME string `json:"job_name" yaml:"job_name" `
	// tags-prefixed, latest-commited
	JOB_TYPE            string `json:"job_type" yaml:"job_type"`
	CHECK_INTERVAL      int    `json:"check_interval" yaml:"check_interval"`
	GIT_REPO_URL        string `json:"git_repo_url" yaml:"git_repo_url"`
	GIT_PRIVATE_KEY     string `json:"git_private_key" yaml:"git_private_key"`
	GIT_START_TAG       string `json:"git_start_tag" yaml:"git_start_tag"`
	GIT_MAX_TAG         string `json:"git_max_tag" yaml:"git_max_tag"`
	GIT_BRANCH          string `json:"git_branch" yaml:"git_branch"`
	GIT_TAG_PREFIX      string `json:"git_tag_prefix" yaml:"git_tag_prefix"`
	GIT_START_TAG_FILE  string `json:"git_start_tag_file" yaml:"git_start_tag_file"`
	LOCAL_GIT_FOLDER    string `json:"local_git_folder" yaml:"local_git_folder"`
	DO_DOCKER_BUILD     bool   `json:"do_docker_build" yaml:"do_docker_build"`
	DOCKER_FILE         string `json:"docker_file" yaml:"docker_file"`
	DOCKER_IMAGE        string `json:"docker_image" yaml:"docker_image"`
	DOCKER_SERVER       string `json:"docker_server" yaml:"docker_server"`
	DOCKER_USER         string `json:"docker_user" yaml:"docker_user"`
	DOCKER_TOKEN        string `json:"docker_token" yaml:"docker_token"`
	DO_SUBFOLDER_SYNC   bool   `json:"do_subfolder_sync" yaml:"do_subfolder_sync"`
	GIT_SUB_FOLDER      string `json:"git_sub_folder" yaml:"git_sub_folder"`
	TARGET_FOLDER       string `json:"target_folder" yaml:"target_folder"`
	DO_MANIFEST_DEPLOY  bool   `json:"do_manifest_deploy" yaml:"do_manifest_deploy"`
	CONTEXT_K8s         string `json:"context_k8s" yaml:"context_k8s"`
	NAMESPACE_K8s       string `json:"namespace_k8s" yaml:"namespace_k8s"`
	DEPLOYMENT_NAME_K8s string `json:"deployment_name_k8s" yaml:"deployment_name_k8s"`
	MANIFESTS_K8S       string `json:"manifests_k8s" yaml:"manifests_k8s"`
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
	checkInterval, err = strconv.Atoi(GetEnvVar("CHECK_INTERVAL", "300"))
	if err != nil {
		checkInterval = 300
	}

	DefaultConfig = Config{
		JOB_NAME:       GetEnvVar("JOB_NAME", "default job"),
		JOB_TYPE:       GetEnvVar("JOB_TYPE", "tags-prefixed"),
		CHECK_INTERVAL: checkInterval,

		GIT_REPO_URL:       GetEnvVar("GIT_REPO_URL", "git@github.com:Direct-Dev-Ru/http2-nodejs-ddru.git"),
		GIT_PRIVATE_KEY:    GetEnvVar("GIT_PRIVATE_KEY", filepath.Join(USER.HomeDir, ".ssh", "id_rsa")),
		GIT_START_TAG:      GetEnvVar("GIT_START_TAG", "v1.0.0"),
		GIT_MAX_TAG:        GetEnvVar("GIT_MAX_TAG", ""),
		GIT_BRANCH:         GetEnvVar("GIT_BRANCH", "main"),
		GIT_TAG_PREFIX:     GetEnvVar("GIT_TAG_PREFIX", "v"),
		GIT_START_TAG_FILE: GetEnvVar("GIT_START_TAG_FILE", "/usr/local/cdddru/start-tag"),
		LOCAL_GIT_FOLDER:   GetEnvVar("LOCAL_GIT_FOLDER", "/tmp/git_local_repo"),

		DO_DOCKER_BUILD: strings.ToLower(GetEnvVar("DO_DOCKER_BUILD", "false")) == "true",
		DOCKER_FILE:     GetEnvVar("DOCKER_FILE", "Dockerfile"),
		DOCKER_IMAGE:    GetEnvVar("DOCKER_IMAGE", "docker.io/kuznetcovay/ddru"),
		DOCKER_SERVER:   GetEnvVar("DOCKER_SERVER", "https://index.docker.io/v1/"),
		DOCKER_USER:     GetEnvVar("DOCKER_USER", ""),
		DOCKER_TOKEN:    GetEnvVar("DOCKER_TOKEN", ""),

		DO_SUBFOLDER_SYNC: strings.ToLower(GetEnvVar("DO_SUBFOLDER_SYNC", "false")) == "true",
		GIT_SUB_FOLDER:    GetEnvVar("GIT_SUB_FOLDER", ""),                                //if empty - all repo to rsync
		TARGET_FOLDER:     GetEnvVar("TARGET_FOLDER", filepath.Join(USER.HomeDir, "app")), //where web app is

		DO_MANIFEST_DEPLOY:  strings.ToLower(GetEnvVar("DO_MANIFEST_DEPLOY", "false")) == "true",
		MANIFESTS_K8S:       GetEnvVar("MANIFESTS_K8S", filepath.Join(USER.HomeDir, "app", "k8s_deployment.yaml")),
		DEPLOYMENT_NAME_K8s: GetEnvVar("DEPLOYMENT_NAME_K8S", "main-site"),
		NAMESPACE_K8s:       GetEnvVar("NAMESPACE_K8S", "test-app"),
		CONTEXT_K8s:         GetEnvVar("CONTEXT_K8S", "default"),
	}

}

type bVerbose bool

// func (verbose bVerbose) vprintln(v ...interface{}) {
// 	if verbose {
// 		fmt.Println(v...)
// 	}
// }

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

type DockerAuths struct {
	Auths map[string]DockerAuth `json:"auths"`
}

type DockerAuth struct {
	Auth string `json:"auth"`
}
