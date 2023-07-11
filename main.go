package main

import (
	"fmt"
	"math"
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
	var logLevel LogLevel = InfoLevel
	configs, err := startup()
	if fbVerbose {
		logLevel = DebugLevel
	}
	CheckIfError(logger, err, true)
	config := configs[0]
	logger = NewLogger(os.Stdout, os.Stderr, logLevel, config.TASK_NAME)

	inlineTest(false, config, logger, true)

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

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		CheckIfError(logger, err, true)
	}
	// how much times retruyin apply manifest
	retryApply := 0
	strMaxTag := ""
	nMaxTag := int64(0)
	nCount := math.MaxInt
	if fbOnce {
		nCount = 1
	}
	for i := 0; i < nCount; i++ {
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
		// here we getting max tag in updated repo to apply
		nMaxTagCandidate, strMaxTagCandidate, err := getMaxTag(repoTags, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX)
		CheckIfError(logger, err, true)

		if retryApply > 0 && nMaxTagCandidate != nMaxTag {
			retryApply = 0
		}
		nMaxTag, strMaxTag = nMaxTagCandidate, strMaxTagCandidate

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

		totalWaitSeconds := 0
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
			PrintInfo(logger, "starting build %s docker image", imageNameTag)
			// building docker image
			err = dockerImageBuild(cli, imageNameTag, config.LOCAL_GIT_FOLDER, config.DOCKER_FILE, logger)
			if err != nil {
				newError := fmt.Errorf("failed to build docker image %s : %v", imageNameTag, err)
				CheckIfError(logger, newError, true)
			}
			// pushing docker image
			PrintInfo(logger, "starting pushing %s docker image", imageNameTag)
			err = dockerImagePush(cli, imageNameTag, config, logger)
			if err != nil {
				newError := fmt.Errorf("failed to push docker image %s : %v", imageNameTag, err)
				CheckIfError(logger, newError, true)
			}
			PrintInfo(logger, "docker image %s builded and pushed successfully", imageNameTag)

			// first thing we next should do - rsync data on external path if specified
			if config.DO_SUBFOLDER_SYNC {
				PrintInfo(logger, "syncing subfolders for release %s", imageNameTag)
				err = rsync(config.TARGET_FOLDER, filepath.Join(config.LOCAL_GIT_FOLDER, config.GIT_SUB_FOLDER))
				CheckIfError(logger, err, true)
			}

			// now it's time to get manifest for k8s deployment
			PrintInfo(logger, "start applying release %s", imageNameTag)
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

			// switch to context
			_, err = runExternalCmd("", "error while switching to context "+config.CONTEXT_K8s, "kubectx", config.CONTEXT_K8s)
			CheckIfError(logger, err, true)

			//  and now we are ready to apply it in our cluster
			// command := []string{"kubectl", "apply", "-f", "-", "--dry-run=client")}
			command := []string{"kubectl", "apply", "-f", "-"}
			outManifestApply, err := runExternalCmd(manifestToApply, "error while applying manifest", command[0], command[1:]...)
			CheckIfError(logger, err, true)

			// well we need wait some time for changes take effect
			if config.CHECK_INTERVAL < 120 {
				config.CHECK_INTERVAL = 120
			}
			checkIntervals := getIntervals(config.CHECK_INTERVAL)

			isReady := false
			retryApply += 1
			for i := 0; i < 5; i++ {
				intervalToWaitSeconds := checkIntervals[i]
				time.Sleep(time.Duration(intervalToWaitSeconds) * time.Second)
				totalWaitSeconds += intervalToWaitSeconds
				// now check applying
				isReady, _ = getDeploymentReadinessStatus(config, imageNameTag)
				if isReady {
					break
				}
				PrintInfo(logger, "kubernetes manifests are not apllying yet for release %s", imageNameTag)
			}

			if isReady {
				currentClusterImageTag, errK8s = getImageTag(config)
				CheckIfError(logger, errK8s, true)

				if currentClusterImageTag == strMaxTag {
					PrintInfo(logger, "release %s applyed successfully \n%v", strMaxTag, outManifestApply)
					gitCurrentTag = strMaxTag
					err = os.WriteFile(pathToStoreStartTag, []byte(gitCurrentTag), 0755)
					CheckIfError(logger, err, false)
					retryApply = 0
				} else {
					PrintInfo(logger, "release %s DO NOT applyed successfully", strMaxTag)
					PrintInfo(logger, "starting attempt number %v to apply release %s", retryApply+1, strMaxTag)
				}
			}

			if retryApply > 3 {
				CheckIfError(logger,
					fmt.Errorf("release %s DO NOT applyed successfully while 3 attempts. exiting", strMaxTag), true)
			}
		}
		if !fbOnce {
			time.Sleep(time.Duration(config.CHECK_INTERVAL-totalWaitSeconds) * time.Second)
		}
	}

}
