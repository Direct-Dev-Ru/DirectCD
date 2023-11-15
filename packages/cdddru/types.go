package cdddru

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

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

type Config struct {
	COMMON CommonConfig `json:"Common" yaml:"Common"`

	GIT GitConfig `json:"Git" yaml:"Git"`

	DOCKER DockerConfig `json:"Docker" yaml:"Docker"`

	DEPLOY DeployConfig `json:"Deploy" yaml:"Deploy"`

	SYNC SyncConfig `json:"Sync" yaml:"Sync"`

	logger *Logger
}

type CommonConfig struct {
	JOB_PATH       string
	JOB_NAME       string `json:"job_name" yaml:"job_name" `
	JOB_TYPE       string `json:"job_type" yaml:"job_type"`
	CHECK_INTERVAL int    `json:"check_interval" yaml:"check_interval"`
	IS_ACTIVE      bool   `json:"is_active,string" yaml:"is_active"`
	VARIABLE_1     string `json:"variable_1" yaml:"variable_1"`
	VARIABLE_2     string `json:"variable_2" yaml:"variable_2"`
	VARIABLE_3     string `json:"variable_3" yaml:"variable_3"`
	VARIABLE_4     string `json:"variable_4" yaml:"variable_4"`
	VARIABLE_5     string `json:"variable_5" yaml:"variable_5"`
	parentLink     *Config
}

type DeployConfig struct {
	DO_MANIFEST_DEPLOY  bool   `json:"do_manifest_deploy,string" yaml:"do_manifest_deploy"`
	DO_WATCH_IMAGE_TAG  bool   `json:"do_watch_image_tag" yaml:"do_watch_image_tag"`
	KUBECONFIG          string `json:"kubeconfig" yaml:"kubeconfig"`
	CONTEXT_K8s         string `json:"context_k8s" yaml:"context_k8s"`
	NAMESPACE_K8s       string `json:"namespace_k8s" yaml:"namespace_k8s"`
	DEPLOYMENT_NAME_K8s string `json:"deployment_name_k8s" yaml:"deployment_name_k8s"`
	MANIFESTS_K8S       string `json:"manifests_k8s" yaml:"manifests_k8s"`
	parentLink          *Config
}

type SyncConfig struct {
	DO_SUBFOLDER_SYNC bool   `json:"do_subfolder_sync,string" yaml:"do_subfolder_sync"`
	GIT_SUB_FOLDER    string `json:"git_sub_folder" yaml:"git_sub_folder"`
	TARGET_FOLDER     string `json:"target_folder" yaml:"target_folder"`
	parentLink        *Config
}

var USER *user.User
var err error
var checkInterval int
var DefaultConfig Config
var DefaultCommonConfig CommonConfig
var DefaultGitConfig GitConfig
var DefaultDockerConfig DockerConfig
var DefaultDeployConfig DeployConfig
var DefaultSyncConfig SyncConfig

func (cfg *Config) SetParentLinks() {
	cfg.COMMON.parentLink = cfg
	cfg.GIT.parentLink = cfg
	cfg.DOCKER.parentLink = cfg
	cfg.DEPLOY.parentLink = cfg
	cfg.SYNC.parentLink = cfg
}

func (cfg *Config) ReplaceConfigFields(content string) (isChanged bool, _ string, err error) {
	contentString := strings.TrimSpace(content)
	isChanged = false
	pattern := `{{ThisConfig:(.*?)}}`
	// Compile the regular expression
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false, "", err
	}
	// Find all matches in the text
	matches := regex.FindAllStringSubmatch(contentString, -1)

	// iterate through matches
	for _, match := range matches {
		replacedText := match[0]
		// fmt.Println(replacedText, match[1])
		replacingText, err := cfg.getFieldValue(match[1])
		if err != nil {
			return false, "", err
		}
		// fmt.Println(value, err)
		contentString = strings.ReplaceAll(contentString, replacedText, replacingText)
		isChanged = true
	}
	return isChanged, contentString, nil
}

func (cfg *Config) getFieldValue(path string) (string, error) {
	fields := strings.Split(path, ":")
	current := reflect.ValueOf(*cfg)

	for _, field := range fields {
		// Check if the field is a valid struct field

		// Get the field value
		fieldValue := current.FieldByName(field)

		// Check if the field exists
		if !fieldValue.IsValid() {
			return "", errors.New("field not found")
		}

		// Update 'current' to the value of the current field
		current = fieldValue
		if current.Kind() == reflect.String {
			fieldValue := current.Interface()
			strValue, ok := fieldValue.(string)
			if !ok {
				return "", errors.New("field not string")
			}
			return strValue, nil
		}
	}

	return "", errors.New("field not found")
}

func init() {

	USER, err = user.Current()
	if err != nil {
		fmt.Printf("failed to get current user: %v\n", err)
		os.Exit(1)
	}
	checkInterval, err = strconv.Atoi(GetEnvVar("CHECK_INTERVAL", "300"))
	if err != nil {
		checkInterval = 360
	}

	DefaultCommonConfig = CommonConfig{
		JOB_NAME:       GetEnvVar("JOB_NAME", "default job"),
		JOB_TYPE:       GetEnvVar("JOB_TYPE", "tags-prefixed"),
		CHECK_INTERVAL: checkInterval,
		VARIABLE_1:     GetEnvVar("CDDDRU_VARIABLE_1", ""),
		VARIABLE_2:     GetEnvVar("CDDDRU_VARIABLE_2", ""),
		VARIABLE_3:     GetEnvVar("CDDDRU_VARIABLE_3", ""),
		VARIABLE_4:     GetEnvVar("CDDDRU_VARIABLE_4", ""),
		VARIABLE_5:     GetEnvVar("CDDDRU_VARIABLE_5", ""),
	}

	DefaultGitConfig = GitConfig{
		GIT_REPO_URL:       GetEnvVar("GIT_REPO_URL", "git@github.com:Direct-Dev-Ru/http2-nodejs-ddru.git"),
		GIT_PRIVATE_KEY:    GetEnvVar("GIT_PRIVATE_KEY", filepath.Join(USER.HomeDir, ".ssh", "id_rsa")),
		GIT_START_TAG:      GetEnvVar("GIT_START_TAG", "v1.0.0"),
		GIT_MAX_TAG:        GetEnvVar("GIT_MAX_TAG", ""),
		GIT_BRANCH:         GetEnvVar("GIT_BRANCH", "main"),
		GIT_TAG_PREFIX:     GetEnvVar("GIT_TAG_PREFIX", "v"),
		GIT_START_TAG_FILE: GetEnvVar("GIT_START_TAG_FILE", "/usr/local/cdddru/start-tag"),
		GIT_LOCAL_FOLDER:   GetEnvVar("GIT_LOCAL_FOLDER", "/tmp/git_local_repo"),
	}

	DefaultDockerConfig = DockerConfig{
		DO_DOCKER_BUILD:  strings.ToLower(GetEnvVar("DO_DOCKER_BUILD", "false")) == "true",
		DOCKER_PLATFORMS: strings.Split(GetEnvVar("DO_DOCKER_BUILD", "linux/amd64"), ","),
		DOCKER_FILE:      GetEnvVar("DOCKER_FILE", "Dockerfile"),
		DOCKER_IMAGE:     GetEnvVar("DOCKER_IMAGE", "docker.io/kuznetcovay/ddru"),
		DOCKER_SERVER:    GetEnvVar("DOCKER_SERVER", "https://index.docker.io/v1/"),
		DOCKER_USER:      GetEnvVar("DOCKER_USER", ""),
		DOCKER_PASSWORD:  GetEnvVar("DOCKER_PASSWORD", ""),
	}

	DefaultDeployConfig = DeployConfig{
		DO_MANIFEST_DEPLOY:  strings.ToLower(GetEnvVar("DO_MANIFEST_DEPLOY", "false")) == "true",
		DO_WATCH_IMAGE_TAG:  strings.ToLower(GetEnvVar("DO_WATCH_IMAGE_TAG", "false")) == "false",
		KUBECONFIG:          GetEnvVar("KUBECONFIG", "/run/configs/kubeconfig/config"),
		MANIFESTS_K8S:       GetEnvVar("MANIFESTS_K8S", filepath.Join(USER.HomeDir, "app", "k8s_deployment.yaml")),
		DEPLOYMENT_NAME_K8s: GetEnvVar("DEPLOYMENT_NAME_K8S", "main-site"),
		NAMESPACE_K8s:       GetEnvVar("NAMESPACE_K8S", "test-app"),
		CONTEXT_K8s:         GetEnvVar("CONTEXT_K8S", "default"),
	}

	DefaultSyncConfig = SyncConfig{
		DO_SUBFOLDER_SYNC: strings.ToLower(GetEnvVar("DO_SUBFOLDER_SYNC", "false")) == "true",
		GIT_SUB_FOLDER:    GetEnvVar("GIT_SUB_FOLDER", ""),                                //if empty - all repo to rsync
		TARGET_FOLDER:     GetEnvVar("TARGET_FOLDER", filepath.Join(USER.HomeDir, "app")), //where web app is

	}

	DefaultConfig = Config{

		COMMON: DefaultCommonConfig,

		GIT: DefaultGitConfig,

		DOCKER: DefaultDockerConfig,

		DEPLOY: DefaultDeployConfig,

		SYNC: DefaultConfig.SYNC,
	}

}
