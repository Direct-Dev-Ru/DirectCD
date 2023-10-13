package cdddru

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/client"
	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"

	ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func RunOneJob(config Config, wg *sync.WaitGroup) {
	logLevel := Tiif(bool(FbVerbose), DebugLevel, InfoLevel).(LogLevel)
	logger := NewLogger(os.Stdout, os.Stderr, logLevel, config.JOB_NAME)
	defer func() {
		wg.Done()
		if v := recover(); v != nil {
			PrintFatal(logger, "job '%v' completes with fatal error: %v", config.JOB_NAME, v)
		}
	}()

	// logger.Debug(fmt.Sprint(PrettyJsonEncodeToString(config)))

	// we trying read saved file with tag applyed if it is not exists we try to detect image version from kubectl
	pathToStoreStartTag := config.GIT_START_TAG_FILE
	if IsStringEmpty(pathToStoreStartTag) {
		pathToStoreStartTag = "/tmp/start-tag-file"
	}
	pathToStoreStartTag += "." + config.JOB_NAME

	err = os.MkdirAll(filepath.Dir(pathToStoreStartTag), 0755)
	CheckIfError(logger, err, true)

	var startImageTag, currentClusterImageTag string
	var errK8s error
	rawStartImageTag, err := os.ReadFile(pathToStoreStartTag)

	if config.DO_MANIFEST_DEPLOY {
		// Here we get from cluster tag version it is currently running
		currentClusterImageTag, errK8s = GetImageTag(config)
		CheckIfError(logger, errK8s, false)
		if errK8s == nil {
			startImageTag = currentClusterImageTag
		} else if err == nil {
			startImageTag = string(rawStartImageTag)
		} else {
			startImageTag = config.GIT_START_TAG
		}
		PrintInfo(logger, "current cluster tag: %v \t current start tag: %v", currentClusterImageTag, startImageTag)
	} else {
		if err == nil {
			startImageTag = string(rawStartImageTag)
		} else {
			startImageTag = config.GIT_START_TAG
		}
		PrintInfo(logger, "current start tag: %v", startImageTag)
	}

	// if downgrade needed - actual start tag is greater than maximum available in job config
	if res, _ := CompareTwoTags(startImageTag, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX); res == -1 {
		startImageTag = config.GIT_TAG_PREFIX + "0.0.0"
	}
	// fmt.Println(111)
	// we write calculated start tag to file for future using
	err = os.WriteFile(pathToStoreStartTag, []byte(startImageTag), 0755)
	CheckIfError(logger, err, true)

	// logger.Debug(fmt.Sprint(startImageTag, " ", pathToStoreStartTag))
	// time.Sleep(300 * time.Second)

	var url, localRepoPath, privateKeyFile string

	url, localRepoPath, privateKeyFile = config.GIT_REPO_URL, CheckFolderPath(config.LOCAL_GIT_FOLDER), config.GIT_PRIVATE_KEY
	// checking existing of private_key_file and halt app if it doesnt exists

	_, err = os.Stat(privateKeyFile)
	if err != nil {
		CheckIfError(logger, fmt.Errorf("read file %s failed %v", privateKeyFile, err), true)
	}

	// Clone the given repository to the given localRepoPath
	PrintInfo(logger, "opening or cloning git repo: %s ...", url)
	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		CheckIfError(logger, fmt.Errorf("generate publickeys failed: %s", err.Error()), true)
	}

	var gitRepository *git.Repository
	var gitWorkTree *git.Worktree

	// Check if git repo exists in localRepoPath and open it, overwise - cloning
	_, err = os.Stat(localRepoPath + ".git")

	if err == nil {
		gitRepository, err = git.PlainOpen(localRepoPath)
		CheckIfError(logger, err, true)
	} else if os.IsNotExist(err) {
		refName := plumbing.NewBranchReferenceName(config.GIT_BRANCH)
		gitRepository, err = git.PlainClone(localRepoPath, false, &git.CloneOptions{
			Auth:          publicKeys,
			URL:           url,
			SingleBranch:  true,
			Progress:      os.Stdout,
			ReferenceName: refName,
		})
		CheckIfError(logger, err, true)
	} else {
		if err != nil {
			CheckIfError(logger, fmt.Errorf("check existing git repo %s failed: %s", localRepoPath, err.Error()), true)
		}
	}

	// Get the git worktree
	gitWorkTree, err = gitRepository.Worktree()
	if err != nil {
		CheckIfError(logger, fmt.Errorf("failed to get worktree: %v", err.Error()), true)
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
	if res, _ := CompareTwoTags(startImageTag, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX); res != 1 {
		if err != nil {
			CheckIfError(logger, fmt.Errorf("current tag: %s is equal or greater than max possible: '%s'", startImageTag, config.GIT_MAX_TAG), true)
		}
	}

	// starting loop here
	gitCurrentTag := startImageTag
	PrintInfo(logger, "start checking for updates ... start git tag version: '%s'", gitCurrentTag)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	CheckIfError(logger, err, true)

	// how much times we try to apply manifest
	retryApply := 0
	strMaxTag := ""
	nMaxTag := int64(0)
	nCount := Tiif(FbOnce, 1, math.MaxInt).(int)

	// err = CheckOutByCommitHash(gitRepository, "d7d8cf2b8026a77cc3afd14143bc3cca05fb5e1e")
	// CheckIfError(logger, err, true)
	// fmt.Println(RunExternalCmd("", "", "sh", "-c", "cd "+config.LOCAL_GIT_FOLDER+"; git checkout d7d8cf2b8026a77cc3afd14143bc3cca05fb5e1e; ls -lah"))

	for i := 0; i < nCount; i++ {
		// retrieving the branch being pointed by HEAD
		// repoRef, err := gitRepository.Head()
		// CheckIfError(logger, err, true)
		// getting the commit hash referenced by current tag
		currentTagsCommitHash, _ := GetCommitHashByTag(gitRepository, gitCurrentTag)

		branchName = "refs/heads/" + config.GIT_BRANCH
		err = gitWorkTree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName(branchName),
			Force:  true,
		})
		if err != nil {
			CheckIfError(logger, fmt.Errorf("failed checkout %s branch: %v", config.GIT_BRANCH, err), true)
		}

		// updating git repository
		err = gitWorkTree.Pull(&git.PullOptions{
			// ReferenceName: repoRef.Name(),
			ReferenceName: plumbing.ReferenceName(branchName),
			RemoteName:    "origin",
			SingleBranch:  true,
			Auth:          publicKeys,
			Force:         true,
		})

		if err != nil && err == git.NoErrAlreadyUpToDate {
			PrintInfo(logger, "git repo at path: %s is up to date", localRepoPath)
		} else if err != nil && err != git.NoErrAlreadyUpToDate {
			CheckIfError(logger, fmt.Errorf("pull error from git lib: %w. trying native git pull", err), false)
			var out string
			// git pull origin main --allow-unrelated-histories
			out, err = RunExternalCmd("", "pull error:", "git", "pull", "origin", branchName, "--allow-unrelated-histories")
			CheckIfError(logger, fmt.Errorf("native git pull error: %w", err), true)
			PrintInfo(logger, "pull out: %s", out)
		} else {
			PrintInfo(logger, "git repo at path: %s pulled successfully, %v", localRepoPath, err)
		}

		// getting all tags from repository after updating
		repoTags, err := GetTagsFromGitRepo(gitRepository, config.GIT_TAG_PREFIX)
		CheckIfError(logger, err, true)

		// getting max tag with specified tag prefix in updated repo to apply
		nMaxTagCandidate, strMaxTagCandidate, err := GetMaxTag(repoTags, config.GIT_MAX_TAG, config.GIT_TAG_PREFIX)
		CheckIfError(logger, err, true)

		if retryApply > 0 && nMaxTagCandidate != nMaxTag {
			retryApply = 0
		}
		nMaxTag, strMaxTag = nMaxTagCandidate, strMaxTagCandidate

		// flag to upgrade or not
		bDoUpgrade := false

		curTag, _ := ConvertTagToNumeric(gitCurrentTag, config.GIT_TAG_PREFIX)
		strMaxTagCommitHash, _ := GetCommitHashByTag(gitRepository, strMaxTag)
		if nMaxTag > curTag {
			bDoUpgrade = true
		}
		if strMaxTag == gitCurrentTag {
			bDoUpgrade = strMaxTagCommitHash != currentTagsCommitHash
		}

		PrintDebug(logger, "\nstrMaxTag: %s, strMaxTagCommitHash: %s,	gitCurrentTag: %s, currentTagsCommitHash: %s, bDoUpgrade: %v", strMaxTag, strMaxTagCommitHash,
			gitCurrentTag, currentTagsCommitHash, bDoUpgrade)

		totalWaitSeconds := 0
		if bDoUpgrade {
			PrintInfo(logger, "starting upgrade for %s release (commit hash: %s)...", strMaxTag, strMaxTagCommitHash)

			// checkout to true tag
			tagFullPath := `refs/tags/` + strMaxTag
			tagRefName := plumbing.ReferenceName(tagFullPath)
			refTag, err := gitRepository.ResolveRevision(plumbing.Revision(tagRefName))
			if err != nil {
				newError := fmt.Errorf("error get reference for tag: %v", err)
				CheckIfError(logger, newError, true)
			}
			err = gitWorkTree.Checkout(&git.CheckoutOptions{
				Hash: *refTag,
			})
			if err != nil {
				newError := fmt.Errorf("error checkout to tag: %v", err)
				CheckIfError(logger, newError, true)
			}
			PrintInfo(logger, "successfull checkout to tag %s hash: %v\n", strMaxTag, refTag)

			imageNameTag := fmt.Sprintf("%s:%s", config.DOCKER_IMAGE, strMaxTag)
			if config.DO_DOCKER_BUILD {
				// building docker image
				PrintInfo(logger, "starting build %s docker image", imageNameTag)
				err = DockerImageBuild(cli, imageNameTag, config.LOCAL_GIT_FOLDER, config.DOCKER_FILE, logger)
				if err != nil {
					newError := fmt.Errorf("failed to build docker image %s : %v", imageNameTag, err)
					CheckIfError(logger, newError, true)
				}
				// pushing docker image
				PrintInfo(logger, "starting pushing %s docker image", imageNameTag)
				err = DockerImagePush(cli, imageNameTag, config, logger)
				if err != nil {
					newError := fmt.Errorf("failed to push docker image %s : %v", imageNameTag, err)
					CheckIfError(logger, newError, true)
				}
				PrintInfo(logger, "docker image %s builded and pushed successfully", imageNameTag)
			}

			// rsync data on target_folder from git_sub_folder if specified
			if config.DO_SUBFOLDER_SYNC {
				PrintInfo(logger, "start sync subfolder for tag %s", strMaxTag)
				err = Rsync(config.TARGET_FOLDER, filepath.Join(config.LOCAL_GIT_FOLDER, config.GIT_SUB_FOLDER))
				CheckIfError(logger, err, true)
				PrintInfo(logger, "successfull sync of subfolder for tag %s", strMaxTag)
			}

			if config.DO_MANIFEST_DEPLOY {
				// now it's time to get final manifest for k8s/k3s deployment from given template
				PrintInfo(logger, "start applying release %s", strMaxTag)
				manifestToApply, err := GenerateManifest(config.MANIFESTS_K8S,
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

				// switch to given context
				_, err = RunExternalCmd("", "error while switching to context "+config.CONTEXT_K8s, "kubectx", config.CONTEXT_K8s)
				CheckIfError(logger, err, true)

				//  now apply it in given cluster
				// command := []string{"kubectl", "apply", "-f", "-", "--dry-run=client")}
				command := []string{"kubectl", "apply", "-f", "-"}
				outManifestApply, err := RunExternalCmd(manifestToApply, "error while applying manifest", command[0], command[1:]...)
				CheckIfError(logger, err, true)

				// waiting some time for changes take effect
				checkIntervals := GetIntervals(config.CHECK_INTERVAL)
				isReady := false
				retryApply += 1
				for i := 0; i < 5; i++ {
					intervalToWaitSeconds := checkIntervals[i]
					time.Sleep(time.Duration(intervalToWaitSeconds) * time.Second)
					totalWaitSeconds += intervalToWaitSeconds
					// now check readiness
					isReady, _ = GetDeploymentReadinessStatus(config, imageNameTag)
					if isReady {
						break
					}
					PrintInfo(logger, "kubernetes manifests are not apllying yet for release %s", imageNameTag)
				}

				if isReady {
					currentClusterImageTag, errK8s = GetImageTag(config)
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
			} // end do manifest apply
		} // end do upgrade
		if !FbOnce {
			time.Sleep(time.Duration(config.CHECK_INTERVAL-totalWaitSeconds) * time.Second)
		}
	}
}
