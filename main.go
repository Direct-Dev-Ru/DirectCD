package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"

	ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func main() {
	var logger *Logger
	configs, err := startup()
	CheckIfError(logger, err, true)
	config := configs[0]
	logger = NewLogger(os.Stdout, os.Stderr, DebugLevel, config.TASK_NAME)
	logger.Debug(fmt.Sprint(PrettyJsonEncodeToString(config)))

	// we trying read saved file with tag applyed if it is not exists we try to detect image version from kubectl
	pathToStoreStartTag := config.GIT_START_TAG_FILE
	if isStringEmpty(pathToStoreStartTag) {
		pathToStoreStartTag = "/tmp/start-tag-file"
	}
	pathToStoreStartTag += "." + config.TASK_NAME

	err = os.MkdirAll(filepath.Dir(pathToStoreStartTag), 0755)
	CheckIfError(logger, err, true)

	var startImageTag string
	rawStartImageTag, err := os.ReadFile(pathToStoreStartTag)

	// Here we ask cluster for tag version it is currently running
	currentClusterImageTag, errK8s := getImageTag(config)
	CheckIfError(logger, errK8s, false)

	PrintInfo(logger, "current cluster tag: %v", currentClusterImageTag)

	if errK8s == nil {
		startImageTag = currentClusterImageTag
	} else if err == nil {
		startImageTag = string(rawStartImageTag)
	} else {
		startImageTag = config.GIT_START_TAG
	}
	// if downgrade needed - actual start tag is greater than maximum available in task config
	if res, _ := compareTwoTags(startImageTag, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX); res == -1 {
		startImageTag = "v0.0.0"
	}
	// we write calculated start tag to file for future using
	err = os.WriteFile(pathToStoreStartTag, []byte(startImageTag), 0755)
	CheckIfError(logger, err, true)

	// logger.Debug(fmt.Sprint(startImageTag, " ", pathToStoreStartTag))
	// time.Sleep(300 * time.Second)

	var url, localRepoPath, privateKeyFile string
	url, localRepoPath, privateKeyFile = config.GIT_REPO_URL, checkFolderPath(config.LOCAL_GIT_FOLDER), config.GIT_PRIVATE_KEY
	// checking existing of private_key_file and halt app if it doesnt exists
	_, err = os.Stat(privateKeyFile)
	if err != nil {
		PrintError(logger, "read file %s failed %s\n", privateKeyFile, err.Error())
		os.Exit(1)
	}

	// Clone the given repository to the given localRepoPath
	PrintInfo(logger, "opening or cloning git repo: %s ...", url)
	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		PrintError(logger, "generate publickeys failed: %s\n", err.Error())
		os.Exit(1)
	}

	var gitRepository *git.Repository
	var gitWorkTree *git.Worktree

	// Check if git repo exists in localRepoPath and open it, overwise - cloning
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

	// Get the git worktree
	gitWorkTree, err = gitRepository.Worktree()
	if err != nil {
		PrintError(logger, "failed to get worktree: %v\n", err.Error())
		os.Exit(1)
	}
	// Checkout the specified branch according to config file
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
	PrintInfo(logger, "start checking for updates ... start git tag version is %s", gitCurrentTag)

	// gitCurrentTag := "v0.0.3"
	// currentTagsCommitHash, err := getCommitHashByTag(gitRepository, gitCurrentTag)
	// if err != nil {
	// 	PrintError(logger, "error for searchig hash of tag %s: %v", gitCurrentTag, err)
	// }
	// PrintInfo(logger, "Tag '%s' has commit hash %s", gitCurrentTag, currentTagsCommitHash)

	// os.Exit(0)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		CheckIfError(logger, err, true)
	}

	for {
		// retrieving the branch being pointed by HEAD
		repoRef, err := gitRepository.Head()
		CheckIfError(logger, err, true)

		// grtting the commit hash referenced by current tag
		currentTagsCommitHash, _ := getCommitHashByTag(gitRepository, gitCurrentTag)

		// updating git repository
		err = gitWorkTree.Pull(&git.PullOptions{
			ReferenceName: repoRef.Name(),
			RemoteName:    "origin",
			Auth:          publicKeys,
			Force:         true,
		})
		if err != nil && err == git.NoErrAlreadyUpToDate {
			PrintInfo(logger, "git repo at path: %s is up to date", localRepoPath)
		} else if err != nil && err != git.NoErrAlreadyUpToDate {
			CheckIfError(logger, err, true)
		} else {
			PrintInfo(logger, "git repo at path: %s pulled successfully, %v", localRepoPath, err)
		}

		// getting tags from repository after updating
		repoTags, err := getTagsFromGitRepo(gitRepository, config.GIT_TAG_PREFIX)
		CheckIfError(logger, err, true)

		nMaxTag, strMaxTag, err := getMaxTag(repoTags, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX)
		CheckIfError(logger, err, true)

		bDoUpgrade := false

		curTag, _ := convertTagToNumeric(gitCurrentTag, config.GIT_TAG_PREFIX)
		strMaxTagCommitHash, _ := getCommitHashByTag(gitRepository, strMaxTag)
		if nMaxTag > curTag {
			bDoUpgrade = true
		}
		if strMaxTag == gitCurrentTag {
			bDoUpgrade = strMaxTagCommitHash != currentTagsCommitHash
		}

		PrintDebug(logger, "\nstrMaxTag: %s, strMaxTagCommitHash: %s,	gitCurrentTag: %s, currentTagsCommitHash: %s, bDoUpgrade: %v", strMaxTag, strMaxTagCommitHash,
			gitCurrentTag, currentTagsCommitHash, bDoUpgrade)

		// if do upgrade
		if bDoUpgrade {
			PrintInfo(logger, "starting upgrade for %s release (commit hash: %s)...", strMaxTag, strMaxTagCommitHash)
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
			PrintInfo(logger, "successfully checked out to tag %s hash: %v\n", strMaxTag, refTag)

			imageNameTag := config.DOCKER_IMAGE + ":" + strMaxTag
			// building docker image
			err = dockerImageBuild(cli, imageNameTag, config.LOCAL_GIT_FOLDER, config.DOCKER_FILE, logger)
			if err != nil {
				newError := fmt.Errorf("failed to build docker image %s : %v", imageNameTag, err)
				CheckIfError(logger, newError, true)
			}
			// pushing docker image
			err = dockerImagePush(cli, imageNameTag, config, logger)
			if err != nil {
				newError := fmt.Errorf("failed to push docker image %s : %v", imageNameTag, err)
				CheckIfError(logger, newError, true)
			}

			// old code through external command docker
			// reading Dockerfile to build app image

			// fullPathToDockerfile := config.DOCKER_FILE
			// if !strings.HasPrefix(fullPathToDockerfile, "/") {
			// 	fullPathToDockerfile = filepath.Join(config.LOCAL_GIT_FOLDER, config.DOCKER_FILE)
			// }
			// dockerfile, err := os.ReadFile(fullPathToDockerfile)
			// if err != nil {
			// 	CheckIfError(logger, err, true)
			// }

			//building Docker image
			// outDockerBuild, err := runExternalCmd(string(dockerfile), "error while building docker image "+imageNameTag,
			// 	"docker", "build", "-t", imageNameTag, "-f", "-", config.LOCAL_GIT_FOLDER)
			// CheckIfError(logger, err, true)
			// PrintInfo(logger, "docker build successfull. output of docker build command for image %s: %v", imageNameTag, outDockerBuild)

			// var errThread bytes.Buffer

			// buildCmd := exec.Command("docker", "build", "-t", imageNameTag, "-f", "-", config.LOCAL_GIT_FOLDER)
			// buildCmd.Stdin = bytes.NewReader(dockerfile)
			// buildCmd.Stderr = &errThread
			// buildCmd.Stdout = os.Stdout

			// err = buildCmd.Run()
			// if err != nil {
			// 	errBuildCmd := fmt.Errorf("failed to execute docker build command: %v (%v)", err, errThread.String())
			// 	CheckIfError(logger, errBuildCmd, true)
			// }

			// pushing Docker image to a registry

			// outDockerPush, err := runExternalCmd(string(dockerfile), "error while pushing docker image "+imageNameTag,
			// 	"docker", "push", imageNameTag)
			// CheckIfError(logger, err, true)
			// PrintInfo(logger, "output of docker push command for image %s: %v", imageNameTag, outDockerPush)

			// pushCmd := exec.Command("docker", "push", imageNameTag)
			// pushCmd.Stdout = os.Stdout
			// pushCmd.Stderr = &errThread

			// err = pushCmd.Run()
			// if err != nil {
			// 	errPushCmd := fmt.Errorf("failed to push docker image %s to registry: %v (%v)",
			// 		imageNameTag, err, errThread.String())
			// 	CheckIfError(logger, errPushCmd, true)
			// }

			PrintInfo(logger, "docker image %v builded and pushed successfully", imageNameTag)

			// first thing we next should do - rsync data on external path if specified
			if config.DO_SUBFOLDER_SYNC {
				err = rsync(config.TARGET_FOLDER, filepath.Join(config.LOCAL_GIT_FOLDER, config.GIT_SUB_FOLDER))
				CheckIfError(logger, err, true)
			}

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
			outManifestApply, err := runExternalCmd(manifestToApply, "error while applying manifest",
				"kubectl", "apply", "-f", "-", "--dry-run=client")
			CheckIfError(logger, err, true)
			PrintInfo(logger, "release %s applyed successfully \n (%v)", strMaxTag, outManifestApply)

			gitCurrentTag = strMaxTag
			err = os.WriteFile(pathToStoreStartTag, []byte(gitCurrentTag), 0755)
			CheckIfError(logger, err, false)

		}

		time.Sleep(time.Duration(config.CHECK_INTERVAL) * time.Second)
	}

}
