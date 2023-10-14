package cdddru

import (
	"fmt"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

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
