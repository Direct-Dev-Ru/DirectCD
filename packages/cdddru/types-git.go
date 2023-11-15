package cdddru

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
	xagent "golang.org/x/crypto/ssh/agent"
)

type GitConfig struct {
	DO_GIT_CLONE       bool   `json:"do_git_clone,string,omitempty" yaml:"do_git_clone"`
	GIT_REPO_URL       string `json:"git_repo_url" yaml:"git_repo_url"`
	GIT_PRIVATE_KEY    string `json:"git_private_key" yaml:"git_private_key"`
	GIT_START_TAG      string `json:"git_start_tag" yaml:"git_start_tag"`
	GIT_MAX_TAG        string `json:"git_max_tag" yaml:"git_max_tag"`
	GIT_TARGET_TAG     string `json:"git_target_tag" yaml:"git_target_tag"`
	GIT_BRANCH         string `json:"git_branch" yaml:"git_branch"`
	GIT_TAG_PREFIX     string `json:"git_tag_prefix" yaml:"git_tag_prefix"`
	GIT_START_TAG_FILE string `json:"git_start_tag_file" yaml:"git_start_tag_file"`
	GIT_LOCAL_FOLDER   string `json:"git_local_folder" yaml:"git_local_folder"`
	branchName         string
	publickeys         *ssh.PublicKeys
	branch             plumbing.ReferenceName
	parentLink         *Config
	needAuth           bool
}

func (gitcfg *GitConfig) AddKeyToSshAgent() (err error) {
	var rawPrivateKeySsh []byte
	cmdout := ""
	logger := gitcfg.parentLink.logger
	if strings.HasPrefix(gitcfg.GIT_REPO_URL, "git") {
		gitcfg.needAuth = true
	}
	if gitcfg.needAuth {
		privateKeyFile := gitcfg.GIT_PRIVATE_KEY
		privateKeyVarName := ""
		if strings.HasPrefix(gitcfg.GIT_PRIVATE_KEY, "VAR:") {
			privateKeyVarName = strings.Split(gitcfg.GIT_PRIVATE_KEY, ":")[1]
			privateKeyFile = ""
		}
		_ = privateKeyVarName

		// if private key from file we should take
		if len(privateKeyFile) > 0 {
			rawPrivateKeySsh, err = os.ReadFile(privateKeyFile)
			if err != nil {
				return fmt.Errorf("read ssh private key file %s failed: %w", privateKeyFile, err)
			}
		} else {
			if k := os.Getenv(privateKeyVarName); len(k) > 0 {
				rawPrivateKeySsh = []byte(k)
			} else {
				return fmt.Errorf("ssh private key? given by var %s is invalid or nil", privateKeyVarName)
			}
		}

		if os.Getenv("SSH_AUTH_SOCK") == "" {
			// Start a new ssh-agent
			cmdout, err := RunExternalCmd("", "", "ssh-agent", "-s")
			if err != nil {
				return fmt.Errorf("getting script for ssh agent failed: %w", err)
			}
			outparts := strings.Split(cmdout, ";")
			outparts = strings.Split(outparts[0], "=")
			os.Setenv(outparts[0], outparts[1])
			PrintDebug(logger, "we use ssh-agent %s", os.Getenv("SSH_AUTH_SOCK"))

		}

		// Connect to the ssh-agent
		var conn net.Conn
		conn, err = net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			return fmt.Errorf("connecting to ssh-agent unix socket failed: %w", err)
		}
		defer conn.Close()

		// Parse the private key
		parsedPrivateKey, err := xssh.ParseRawPrivateKey(rawPrivateKeySsh)
		if err != nil {
			return fmt.Errorf("parsing ssh private key file %s failed: %w", privateKeyFile, err)
		}
		agentClient := xagent.NewClient(conn)
		if agentClient == nil {
			return fmt.Errorf("ssh agent client init failed")
		}
		// Add the private key to the ssh-agent
		err = agentClient.Add(xagent.AddedKey{
			PrivateKey: parsedPrivateKey,
		})
		if err != nil {
			return fmt.Errorf("parsing ssh private key file %s failed: %w", privateKeyFile, err)
		}

		cmdout, err = RunExternalCmd("", "list ssh-agent failed", "ssh-add", "-l")
		CheckIfErrorFmt(logger, err, fmt.Errorf("add key to ssh-agent failed: %w", err), false)
		PrintInfo(logger, "%s", cmdout)
	}
	return nil
}

func (gitcfg *GitConfig) OpenOrCloneRepo(url string, logger *Logger) (gitRepository *git.Repository, gitWorkTree *git.Worktree, err error) {

	PrintInfo(logger, "opening or cloning git repo: %s ...", url)
	gitcfg.publickeys, err = ssh.NewPublicKeysFromFile("git", gitcfg.GIT_PRIVATE_KEY, "")
	if err != nil {
		err = fmt.Errorf("generate publickeys failed: %w", err)
		return
		// CheckIfError(logger, fmt.Errorf("generate publickeys failed: %w", err), true)
	}

	// Check if git repo exists in localRepoPath and open it, overwise - cloning
	_, err = os.Stat(filepath.Join(gitcfg.GIT_LOCAL_FOLDER, ".git"))
	gitcfg.branchName = fmt.Sprintf("refs/heads/%s", gitcfg.GIT_BRANCH)

	refName := plumbing.NewBranchReferenceName(gitcfg.GIT_BRANCH)
	gitcfg.branch = refName

	if err == nil {
		gitRepository, err = git.PlainOpen(gitcfg.GIT_LOCAL_FOLDER)
		if err != nil {
			err = fmt.Errorf("opening repository failed: %w", err)
			return
		}
		// CheckIfError(logger, err, true)
	} else if os.IsNotExist(err) {
		gitRepository, err = git.PlainClone(gitcfg.GIT_LOCAL_FOLDER, false, &git.CloneOptions{
			Auth:          gitcfg.publickeys,
			URL:           url,
			SingleBranch:  true,
			Progress:      os.Stdout,
			ReferenceName: refName,
		})
		if err != nil {
			err = fmt.Errorf("cloning repository failed: %w", err)
			return
		}
		// CheckIfError(logger, err, true)
	} else {
		if err != nil {
			err = fmt.Errorf("checking existing git repo %s failed: %s", gitcfg.GIT_LOCAL_FOLDER, err)
			return
			// CheckIfError(logger, fmt.Errorf("check existing git repo %s failed: %s", gitcfg.GIT_LOCAL_FOLDER, err.Error()), true)
		}
	}

	// Get the git worktree
	gitWorkTree, err = gitRepository.Worktree()
	if err != nil {
		err = fmt.Errorf("getting worktree failed: %w", err)
		return
		// CheckIfError(logger, fmt.Errorf("failed to get worktree: %v", err.Error()), true)
	}
	return
}

func (gitcfg *GitConfig) CliPull(logger *Logger) (err error) {
	err = os.Chdir(gitcfg.GIT_LOCAL_FOLDER)
	if err != nil {
		return fmt.Errorf("pulling git repository failed: %w", err)
	}
	var stdout string
	stdout, err = RunExternalCmd("", "pulling error:", "git", "pull", "-f", "--tags", "origin", gitcfg.GIT_BRANCH)
	PrintInfo(logger, "%s", stdout)
	return err
}

func (gitcfg *GitConfig) Pull(gitWorkTree *git.Worktree, logger *Logger) (err error) {

	err = gitWorkTree.Pull(&git.PullOptions{
		// ReferenceName: repoRef.Name(),
		ReferenceName: plumbing.ReferenceName(gitcfg.branchName),
		RemoteName:    "origin",
		SingleBranch:  true,
		Auth:          gitcfg.publickeys,
		Force:         true,
	})

	if err != nil && err == git.NoErrAlreadyUpToDate {
		PrintInfo(logger, "git repo at path: %s is up to date", gitcfg.GIT_LOCAL_FOLDER)
		return nil
	} else if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("pulling git repository failed: %w", err)

		// CheckIfError(logger, fmt.Errorf("pull error from git lib: %w. trying native git pull", err), false)

		// var out string
		// git pull origin main --allow-unrelated-histories
		// out, err = RunExternalCmd("", "pull error:", "git", "pull", "origin", config.GIT.GIT_BRANCH, "--allow-unrelated-histories")
		// CheckIfError(logger, fmt.Errorf("native git pull error: %w", err), true)

		// PrintInfo(logger, "pull out: %s", out)

	} else {
		PrintInfo(logger, "git repo at path: %s pulled successfully, %v", gitcfg.GIT_LOCAL_FOLDER, err)
		return nil
	}
}
