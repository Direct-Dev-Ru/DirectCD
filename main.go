package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"

	ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

var repoTags []string
var logger *Logger

func main() {

	CheckArgs(false, "config-file")
	config, err := startup()
	logger = NewLogger(os.Stdout, os.Stderr, DebugLevel, config.TASK_NAME)
	CheckIfError(logger, err, false)

	logger.Debug(fmt.Sprint(PrettyJsonEncodeToString(config)))

	// we trying read saved file with tag applyed if it is not exists we try to detect image version from kubectl
	pathToStoreStartTag := config.GIT_START_TAG_FILE
	if isStringEmpty(pathToStoreStartTag) {
		pathToStoreStartTag = "/tmp/start-tag-file"
	}

	err = os.MkdirAll(filepath.Dir(pathToStoreStartTag), 0755)
	CheckIfError(logger, err, true)

	var startImageTag string
	rawStartImageTag, err := os.ReadFile(pathToStoreStartTag)
	// Here we ask cluster for tag version it is currently running
	currentClusterImageTag, errK8s := getImageTag(config)
	CheckIfError(logger, errK8s, false)
	if errK8s == nil {
		startImageTag = currentClusterImageTag
	} else if err == nil {
		startImageTag = string(rawStartImageTag)
	} else {
		startImageTag = config.GIT_START_TAG
	}
	err = os.WriteFile(pathToStoreStartTag, []byte(startImageTag), 0755)
	CheckIfError(logger, err, true)

	logger.Debug(fmt.Sprint(startImageTag, " ", pathToStoreStartTag))
	// time.Sleep(300 * time.Second)

	repoTags = make([]string, 0, 4)
	var url, localRepoPath, privateKeyFile string
	url, localRepoPath, privateKeyFile = config.GIT_REPO_URL, checkFolderPath(config.LOCAL_GIT_FOLDER), config.GIT_PRIVATE_KEY
	// checking existing of private_key_file
	_, err = os.Stat(privateKeyFile)
	if err != nil {
		PrintWarning(logger, "read file %s failed %s\n", privateKeyFile, err.Error())
		os.Exit(1)
	}
	// Clone the given repository to the given localRepoPath
	PrintInfo(logger, "open or clone git repo: %s ", url)
	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		PrintWarning(logger, "generate publickeys failed: %s\n", err.Error())
		return
	}
	var gitRepository *git.Repository
	var gitWorkTree *git.Worktree

	// Check if repo is in localRepoPath
	_, err = os.Stat(localRepoPath + ".git")
	if err == nil {
		gitRepository, err = git.PlainOpen(localRepoPath)
		CheckIfError(logger, err, true)
	} else if os.IsNotExist(err) {
		gitRepository, err = git.PlainClone(localRepoPath, false, &git.CloneOptions{
			Auth:     publicKeys,
			URL:      url,
			Progress: os.Stdout,
		})
		CheckIfError(logger, err, true)
	} else {
		PrintError(logger, "check existing git repo %s failed: %s\n", localRepoPath, err.Error())
		os.Exit(1)
	}
	// Get the worktree
	gitWorkTree, err = gitRepository.Worktree()
	if err != nil {
		PrintError(logger, "failed to get worktree: %v\n", err.Error())
		os.Exit(1)
	}
	// Checkout the cpecified in config file branch
	branchName := "refs/heads/" + config.GIT_BRANCH
	err = gitWorkTree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchName),
		Force:  true,
	})
	if err != nil {
		CheckIfError(logger, fmt.Errorf("failed checkout %s branch: %v", config.GIT_BRANCH, err), true)
	}

	// if start tag is equal or greater than max possible tag given in config -> exit tool
	if res, _ := compareTwoTags(startImageTag, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX); res != 1 {
		logger.Error(fmt.Sprintf("current tag: %s is equal or greater than max possible: %s", startImageTag, config.GIT_MAX_TAG))
		os.Exit(1)
	}

	// starting loop here
	gitCurrentTag := startImageTag
	PrintInfo(logger, "starting git tag version is %s", gitCurrentTag)
	for {

		// ... retrieving the branch being pointed by HEAD
		repoRef, err := gitRepository.Head()
		CheckIfError(logger, err, true)

		err = gitWorkTree.Pull(&git.PullOptions{
			ReferenceName: repoRef.Name(),
			RemoteName:    "origin",
			Auth:          publicKeys,
			Force:         true,
		})
		if err != nil && err == git.NoErrAlreadyUpToDate {
			PrintInfo(logger, "git repo at path: %s is up to date", localRepoPath)
		}
		if err != nil && err != git.NoErrAlreadyUpToDate {
			CheckIfError(logger, err, true)
		}

		tagRefs, err := gitRepository.Tags()
		if err != nil {
			PrintError(logger, "failed to get tags from repository: %v", err)
		}
		// Iterate over the tags and print their names
		tagRefs.ForEach(func(tagRef *plumbing.Reference) error {
			if strings.HasPrefix(tagRef.Name().Short(), config.GIT_TAG_PREFIX) {
				repoTags = append(repoTags, tagRef.Name().Short())
			}
			return nil
		})

		nMaxTag, strMaxTag, err := getMaxTag(repoTags, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX)
		CheckIfError(logger, err, true)

		if curTag, _ := convertTagToNumeric(gitCurrentTag, config.GIT_TAG_PREFIX); nMaxTag > curTag {
			PrintInfo(logger, "starting upgrade for %s release ...", strMaxTag)
			// Get the reference for the tag
			tagFullPath := `refs/tags/` + strMaxTag
			tagRefName := plumbing.ReferenceName(tagFullPath)
			refTag, err := gitRepository.ResolveRevision(plumbing.Revision(tagRefName))
			if err != nil {
				errNew := fmt.Errorf("failed to get reference for tag: %v", err)
				CheckIfError(logger, errNew, true)
			}

			err = gitWorkTree.Checkout(&git.CheckoutOptions{
				Hash: *refTag,
			})
			if err != nil {
				errNew := fmt.Errorf("failed to checkout tag: %v", err)
				CheckIfError(logger, errNew, true)
			}
			PrintInfo(logger, "checked out to tag %s hash: %v\n", strMaxTag, refTag)

			// Read the Dockerfile
			dockerfile, err := os.ReadFile(filepath.Join(config.LOCAL_GIT_FOLDER, "Dockerfile"))
			if err != nil {
				CheckIfError(logger, err, true)
			}
			// Build the Docker image

			// Create a buffer for stderr
			var errThread bytes.Buffer
			imageNameTag := config.DOCKER_IMAGE + ":" + strMaxTag
			buildCmd := exec.Command("docker", "build", "-t", imageNameTag, "-f", "-", config.LOCAL_GIT_FOLDER)

			buildCmd.Stdin = bytes.NewReader(dockerfile)
			buildCmd.Stderr = &errThread
			buildCmd.Stdout = os.Stdout

			// outBuild, err := buildCmd.Output()
			err = buildCmd.Run()
			if err != nil {
				errBuildCmd := fmt.Errorf("failed to execute docker build command: %v (%v)", err, errThread.String())
				CheckIfError(logger, errBuildCmd, true)
			}
			// Push the Docker image to a registry
			pushCmd := exec.Command("docker", "push", imageNameTag)
			pushCmd.Stdout = os.Stdout
			buildCmd.Stderr = &errThread

			err = pushCmd.Run()
			if err != nil {
				errPushCmd := fmt.Errorf("failed to push docker image %s to registry: %v (%v)",
					imageNameTag, err, errThread.String())
				CheckIfError(logger, errPushCmd, true)
			}

			PrintInfo(logger, "docker image %v builded successfully", imageNameTag)
			// now it's time to get manifest for k8s deployment
			manifestToApply, err := generateManifest(config.MANIFESTS_K8S,
				struct {
					Release   string
					Image     string
					PgSecrets string
				}{
					Release:   strMaxTag,
					Image:     imageNameTag,
					PgSecrets: "/root/.config/pg",
				})
			CheckIfError(logger, err, true)
			//  and now we are ready to apply it in our cluster
			// first thing we do - rsync data on external path if specified
			err = rsync(config.TARGET_FOLDER, filepath.Join(config.LOCAL_GIT_FOLDER, config.GIT_SUB_FOLDER))
			CheckIfError(logger, err, true)
			outManifestApply, err := runExternalCmd(manifestToApply, "error while applying manifest", "kubectl", "apply", "-f", "-")
			CheckIfError(logger, err, true)
			gitCurrentTag = strMaxTag
			err = os.WriteFile(pathToStoreStartTag, []byte(gitCurrentTag), 0755)
			CheckIfError(logger, err, false)
			PrintInfo(logger, "release %s applyed successfully (%v)", gitCurrentTag, outManifestApply)
		}

		time.Sleep(time.Duration(config.CHECK_INTERVAL) * time.Second)
	}

}
