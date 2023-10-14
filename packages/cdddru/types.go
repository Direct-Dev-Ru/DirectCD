package cdddru

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Config struct {
	JOB_NAME string `json:"job_name" yaml:"job_name" `
	// tags-prefixed, latest-commited
	JOB_TYPE       string `json:"job_type" yaml:"job_type"`
	CHECK_INTERVAL int    `json:"check_interval" yaml:"check_interval"`

	// GIT_REPO_URL       string `json:"git_repo_url" yaml:"git_repo_url"`
	// GIT_PRIVATE_KEY    string `json:"git_private_key" yaml:"git_private_key"`
	// GIT_START_TAG      string `json:"git_start_tag" yaml:"git_start_tag"`
	// GIT_MAX_TAG        string `json:"git_max_tag" yaml:"git_max_tag"`
	// GIT_BRANCH         string `json:"git_branch" yaml:"git_branch"`
	// GIT_TAG_PREFIX     string `json:"git_tag_prefix" yaml:"git_tag_prefix"`
	// GIT_START_TAG_FILE string `json:"git_start_tag_file" yaml:"git_start_tag_file"`
	// LOCAL_GIT_FOLDER   string `json:"local_git_folder" yaml:"local_git_folder"`

	GIT GitConfig `json:"Git" yaml:"Git"`

	DO_DOCKER_BUILD bool   `json:"do_docker_build" yaml:"do_docker_build"`
	DOCKER_FILE     string `json:"docker_file" yaml:"docker_file"`
	DOCKER_IMAGE    string `json:"docker_image" yaml:"docker_image"`
	DOCKER_SERVER   string `json:"docker_server" yaml:"docker_server"`
	DOCKER_USER     string `json:"docker_user" yaml:"docker_user"`
	DOCKER_TOKEN    string `json:"docker_token" yaml:"docker_token"`

	DO_SUBFOLDER_SYNC bool   `json:"do_subfolder_sync" yaml:"do_subfolder_sync"`
	GIT_SUB_FOLDER    string `json:"git_sub_folder" yaml:"git_sub_folder"`
	TARGET_FOLDER     string `json:"target_folder" yaml:"target_folder"`

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
var DefaultGitConfig GitConfig

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

	DefaultGitConfig = GitConfig{
		GIT_REPO_URL:       GetEnvVar("GIT_REPO_URL", "git@github.com:Direct-Dev-Ru/http2-nodejs-ddru.git"),
		GIT_PRIVATE_KEY:    GetEnvVar("GIT_PRIVATE_KEY", filepath.Join(USER.HomeDir, ".ssh", "id_rsa")),
		GIT_START_TAG:      GetEnvVar("GIT_START_TAG", "v1.0.0"),
		GIT_MAX_TAG:        GetEnvVar("GIT_MAX_TAG", ""),
		GIT_BRANCH:         GetEnvVar("GIT_BRANCH", "main"),
		GIT_TAG_PREFIX:     GetEnvVar("GIT_TAG_PREFIX", "v"),
		GIT_START_TAG_FILE: GetEnvVar("GIT_START_TAG_FILE", "/usr/local/cdddru/start-tag"),
		LOCAL_GIT_FOLDER:   GetEnvVar("LOCAL_GIT_FOLDER", "/tmp/git_local_repo"),
	}

	DefaultConfig = Config{
		JOB_NAME:       GetEnvVar("JOB_NAME", "default job"),
		JOB_TYPE:       GetEnvVar("JOB_TYPE", "tags-prefixed"),
		CHECK_INTERVAL: checkInterval,

		GIT: DefaultGitConfig,

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

type GitConfig struct {
	DO_GIT_CLONE       bool   `json:"do_git_clone,string,omitempty" yaml:"do_git_clone"`
	GIT_REPO_URL       string `json:"git_repo_url" yaml:"git_repo_url"`
	GIT_PRIVATE_KEY    string `json:"git_private_key" yaml:"git_private_key"`
	GIT_START_TAG      string `json:"git_start_tag" yaml:"git_start_tag"`
	GIT_MAX_TAG        string `json:"git_max_tag" yaml:"git_max_tag"`
	GIT_BRANCH         string `json:"git_branch" yaml:"git_branch"`
	GIT_TAG_PREFIX     string `json:"git_tag_prefix" yaml:"git_tag_prefix"`
	GIT_START_TAG_FILE string `json:"git_start_tag_file" yaml:"git_start_tag_file"`
	LOCAL_GIT_FOLDER   string `json:"local_git_folder" yaml:"local_git_folder"`
	branchName         string
	publickeys         *ssh.PublicKeys
	branch             plumbing.ReferenceName
}

func (gitcfg *GitConfig) OpenOrCloneRepo(url string, logger *Logger) (gitRepository *git.Repository, gitWorkTree *git.Worktree, err error) {

	PrintInfo(logger, "opening or cloning git repo: %s ...", url)
	gitcfg.publickeys, err = ssh.NewPublicKeysFromFile("git", gitcfg.GIT_PRIVATE_KEY, "")
	if err != nil {
		return
		// CheckIfError(logger, fmt.Errorf("generate publickeys failed: %w", err), true)
	}

	// Check if git repo exists in localRepoPath and open it, overwise - cloning
	_, err = os.Stat(filepath.Join(gitcfg.LOCAL_GIT_FOLDER, ".git"))
	gitcfg.branchName = fmt.Sprintf("refs/heads/%s", gitcfg.GIT_BRANCH)

	refName := plumbing.NewBranchReferenceName(gitcfg.GIT_BRANCH)
	gitcfg.branch = refName

	if err == nil {
		gitRepository, err = git.PlainOpen(gitcfg.LOCAL_GIT_FOLDER)
		if err != nil {
			return
		}
		// CheckIfError(logger, err, true)
	} else if os.IsNotExist(err) {
		gitRepository, err = git.PlainClone(gitcfg.LOCAL_GIT_FOLDER, false, &git.CloneOptions{
			Auth:          gitcfg.publickeys,
			URL:           url,
			SingleBranch:  true,
			Progress:      os.Stdout,
			ReferenceName: refName,
		})
		if err != nil {
			return
		}
		// CheckIfError(logger, err, true)
	} else {
		if err != nil {
			return
			// CheckIfError(logger, fmt.Errorf("check existing git repo %s failed: %s", gitcfg.LOCAL_GIT_FOLDER, err.Error()), true)
		}
	}

	// Get the git worktree
	gitWorkTree, err = gitRepository.Worktree()
	if err != nil {
		return
		// CheckIfError(logger, fmt.Errorf("failed to get worktree: %v", err.Error()), true)
	}
	return
}

func (gitcfg *GitConfig) Pull(gitWorkTree *git.Worktree, logger *Logger) error {

	var err error = gitWorkTree.Pull(&git.PullOptions{
		// ReferenceName: repoRef.Name(),
		ReferenceName: plumbing.ReferenceName(gitcfg.branchName),
		RemoteName:    "origin",
		SingleBranch:  true,
		Auth:          gitcfg.publickeys,
		Force:         true,
	})

	if err != nil && err == git.NoErrAlreadyUpToDate {
		PrintInfo(logger, "git repo at path: %s is up to date", gitcfg.LOCAL_GIT_FOLDER)
		return nil
	} else if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("pull error from git lib: %w", err)

		// CheckIfError(logger, fmt.Errorf("pull error from git lib: %w. trying native git pull", err), false)

		// var out string
		// git pull origin main --allow-unrelated-histories
		// out, err = RunExternalCmd("", "pull error:", "git", "pull", "origin", config.GIT.GIT_BRANCH, "--allow-unrelated-histories")
		// CheckIfError(logger, fmt.Errorf("native git pull error: %w", err), true)

		// PrintInfo(logger, "pull out: %s", out)

	} else {
		PrintInfo(logger, "git repo at path: %s pulled successfully, %v", gitcfg.LOCAL_GIT_FOLDER, err)
		return nil
	}
}

type DockerConfig struct {
	DO_DOCKER_BUILD bool   `json:"do_docker_build,string,omitempty" yaml:"do_docker_build"`
	DOCKER_FILE     string `json:"docker_file" yaml:"docker_file"`
	DOCKER_IMAGE    string `json:"docker_image" yaml:"docker_image"`
	DOCKER_SERVER   string `json:"docker_server" yaml:"docker_server"`
	DOCKER_USER     string `json:"docker_user" yaml:"docker_user"`
	DOCKER_TOKEN    string `json:"docker_token" yaml:"docker_token"`
}

type DeployConfig struct {
	DO_MANIFEST_DEPLOY  bool   `json:"do_manifest_deploy,string,omitempty" yaml:"do_manifest_deploy"`
	CONTEXT_K8s         string `json:"context_k8s" yaml:"context_k8s"`
	NAMESPACE_K8s       string `json:"namespace_k8s" yaml:"namespace_k8s"`
	DEPLOYMENT_NAME_K8s string `json:"deployment_name_k8s" yaml:"deployment_name_k8s"`
	MANIFESTS_K8S       string `json:"manifests_k8s" yaml:"manifests_k8s"`
}

type SyncConfig struct {
	DO_SUBFOLDER_SYNC bool   `json:"do_subfolder_sync,string,omitempty" yaml:"do_subfolder_sync"`
	GIT_SUB_FOLDER    string `json:"git_sub_folder" yaml:"git_sub_folder"`
	TARGET_FOLDER     string `json:"target_folder" yaml:"target_folder"`
}
