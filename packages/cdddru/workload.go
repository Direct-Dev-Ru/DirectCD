package cdddru

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	// memory "github.com/go-git/go-git/v5/storage/memory"
)

func RunOneJob(config *Config, wg *sync.WaitGroup) {
	var err error
	var originalSHA, currentSHA string

	logLevel := Tiif(bool(FbVerbose), DebugLevel, InfoLevel).(LogLevel)
	logger := NewLogger(os.Stdout, os.Stderr, logLevel, config.COMMON.JOB_NAME)
	config.logger = logger
	defer func() {
		wg.Done()
		if v := recover(); v != nil {
			PrintFatal(logger, "job '%v' completes with fatal error: %v", config.COMMON.JOB_NAME, v)
		}
	}()
	if !config.COMMON.IS_ACTIVE {
		PrintInfo(logger, "job %s is not active", config.COMMON.JOB_NAME)
		return
	}
	originalSHA, err = CalculateSHA256(config.COMMON.JOB_PATH)
	if err != nil {
		CheckIfErrorFmt(logger, err, fmt.Errorf("calculate SHA256 error: %w", err), true)
	}

	logger.Debug(fmt.Sprint(PrettyJsonEncodeToString(config)))

	// we trying read saved file with tag applyed if it is not exists we try to detect image version from kubectl
	pathToStoreStartTag := config.GIT.GIT_START_TAG_FILE
	if IsStringEmpty(pathToStoreStartTag) {
		pathToStoreStartTag = "/tmp/start-tag-file"
	}
	pathToStoreStartTag += "." + config.COMMON.JOB_NAME

	err = os.MkdirAll(filepath.Dir(pathToStoreStartTag), 0700)
	CheckIfError(logger, err, false)

	var startImageTag, currentClusterImageTag string
	var errK8s error
	rawStartImageTag, err := os.ReadFile(pathToStoreStartTag)
	CheckIfError(logger, err, false)

	if config.DEPLOY.DO_MANIFEST_DEPLOY {
		// Here we get from cluster tag version it is currently running
		currentClusterImageTag, errK8s = GetImageTag(config)
		CheckIfError(logger, errK8s, false)
		if errK8s == nil {
			startImageTag = currentClusterImageTag
		} else if err == nil {
			startImageTag = string(rawStartImageTag)
		} else {
			startImageTag = config.GIT.GIT_START_TAG
		}
		PrintInfo(logger, "current cluster tag: %v \t current start tag: %v", currentClusterImageTag, startImageTag)
	} else {
		if err == nil {
			startImageTag = string(rawStartImageTag)
		} else {
			startImageTag = config.GIT.GIT_START_TAG
		}
		PrintInfo(logger, "current start tag: %v", startImageTag)
	}

	// if downgrade needed - actual start tag is greater than maximum available in job config
	if res, _ := CompareTwoTags(startImageTag, config.GIT.GIT_MAX_TAG, config.GIT.GIT_TAG_PREFIX); res == -1 {
		startImageTag = config.GIT.GIT_TAG_PREFIX + "0.0.0"
	}

	// we write calculated start tag to file for future using
	err = os.WriteFile(pathToStoreStartTag, []byte(startImageTag), 0600)
	CheckIfError(logger, err, true)

	var url, privateKeyFile string

	url, privateKeyFile = config.GIT.GIT_REPO_URL, config.GIT.GIT_PRIVATE_KEY
	_ = privateKeyFile

	// checking existing of private_key_file and halt app if it doesnt exists or other error
	// but only if url don't starts from http protocol

	// Clone the given repository to the given localRepoPath
	var gitRepository *git.Repository
	var gitWorkTree *git.Worktree

	err = config.GIT.AddKeyToSshAgent()
	if err != nil {
		CheckIfError(logger, err, true)
		return
	}

	// do init open or clone git repo if we set it in job config
	if config.GIT.DO_GIT_CLONE {
		gitRepository, gitWorkTree, err = config.GIT.OpenOrCloneRepo(url, logger)
		if err != nil {
			CheckIfError(logger, fmt.Errorf("opening or cloning repo %s failed: %s", url, err.Error()), true)
		}

		err = gitWorkTree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName(config.GIT.branchName),
			Force:  true,
		})
		if err != nil {
			CheckIfError(logger, fmt.Errorf("failed checkout %s branch: %v", config.GIT.GIT_BRANCH, err), true)
		}

		// if start tag is equal or greater than max possible tag given in config -> exit job
		if res, _ := CompareTwoTags(startImageTag, config.GIT.GIT_MAX_TAG, config.GIT.GIT_TAG_PREFIX); res != 1 {
			PrintFatal(logger, "current tag: %s is equal or greater than max possible (exiting): '%s'", startImageTag, config.GIT.GIT_MAX_TAG)
			return
		}
	}

	// starting loop here
	gitCurrentTag := startImageTag
	PrintInfo(logger, "start checking for updates => start git tag version: %s", gitCurrentTag)

	// cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	// CheckIfError(logger, err, true)

	// how much times we try to apply manifest
	retryApply := 0
	strMaxTag := ""
	nMaxTag := int64(0)
	nCount := Tiif(FbOnce, 1, math.MaxInt).(int)

	// if we do git clone and pull to check for app versions
	if config.GIT.DO_GIT_CLONE {
		for i := 0; i < nCount; i++ {
			currentTagsCommitHash, _ := GetCommitHashByTag(gitRepository, gitCurrentTag)
			// checkout to branch given in config
			err = gitWorkTree.Checkout(&git.CheckoutOptions{
				Branch: plumbing.ReferenceName(config.GIT.branchName),
				Force:  true,
			})
			if err != nil {
				CheckIfError(logger, fmt.Errorf("failed checkout %s branch: %v", config.GIT.GIT_BRANCH, err), true)
			}

			// updating git repository
			// err = config.GIT.Pull(gitWorkTree, logger)
			err = config.GIT.CliPull(logger)
			if err != nil {
				CheckIfError(logger, fmt.Errorf("git pull failed: %s", err.Error()), true)
			}

			// getting all tags from repository after updating
			repoTags, err := GetTagsFromGitRepo(gitRepository, config.GIT.GIT_TAG_PREFIX)
			CheckIfError(logger, err, true)

			// getting max tag with specified tag prefix in updated repo to apply
			nMaxTagCandidate, strMaxTagCandidate, err := GetMaxTag(repoTags, config.GIT.GIT_MAX_TAG, config.GIT.GIT_TAG_PREFIX)
			CheckIfErrorFmt(logger, err, fmt.Errorf("getting max tag failed: %w", err), true)

			if retryApply > 0 && nMaxTagCandidate != nMaxTag {
				retryApply = 0
			}
			nMaxTag, strMaxTag = nMaxTagCandidate, strMaxTagCandidate

			// flag to upgrade or not
			bDoUpgrade := false

			curTag, err := ConvertTagToNumeric(gitCurrentTag, config.GIT.GIT_TAG_PREFIX)
			CheckIfErrorFmt(logger, err, fmt.Errorf("convert tag to numeric failed: %w", err), false)

			strMaxTagCommitHash, err := GetCommitHashByTag(gitRepository, strMaxTag)
			CheckIfErrorFmt(logger, err, fmt.Errorf("getting commit hash failed: %w", err), false)

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

				tagRefName := plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", strMaxTag))

				refTag, err := gitRepository.ResolveRevision(plumbing.Revision(tagRefName))
				if err != nil {
					newError := fmt.Errorf("error get reference for tag: %v", err)
					CheckIfError(logger, newError, true)
				}
				err = gitWorkTree.Checkout(&git.CheckoutOptions{
					Hash: *refTag,
				})
				CheckIfErrorFmt(logger, err, fmt.Errorf("error checkout to tag: %v", err), true)

				PrintInfo(logger, "successfully checkout to tag %s hash: %v\n", strMaxTag, refTag)

				// final docker image name with tag
				imageNameTag := fmt.Sprintf("%s:%s", config.DOCKER.DOCKER_IMAGE, strMaxTag)

				// TODO: move this parameter to config
				var platforms []string = []string{"linux/arm64", "linux/amd64"}
				
				// if we say in config to do docker build
				if config.DOCKER.DO_DOCKER_BUILD {

					config.DOCKER.SetAuth("/run/configs/dockerconfig/")
					PrintInfo(logger, "starting building image %s", imageNameTag)

					// we use buildx to make multy-arch image
					err = config.DOCKER.DockerImageBuildx(imageNameTag, config.GIT.LOCAL_GIT_FOLDER, platforms, logger)
					if e := CheckIfErrorFmt(logger, err, fmt.Errorf("building image %s failed: %w", imageNameTag, err), true); e != nil {
						PrintInfo(logger, "job %s failed and will be closed", config.COMMON.JOB_NAME)
						return
					}
					gitCurrentTag = strMaxTag
					err = os.WriteFile(pathToStoreStartTag, []byte(gitCurrentTag), 0600)
					CheckIfErrorFmt(logger, err, fmt.Errorf("write to file with start image failed: %w", err), false)

					PrintInfo(logger, "successfully build image %s", imageNameTag)

					// building docker image for different platforms
					// for _, platform := range platforms {
					// 	PrintInfo(logger, "starting build %s docker image for %s platform", imageNameTag, platform)
					// 	err = DockerImageBuild(cli, imageNameTag, config.GIT.LOCAL_GIT_FOLDER, config.DOCKER.DOCKER_FILE, platform, logger)
					// 	if err != nil {
					// 		newError := fmt.Errorf("failed to build docker image %s for platform %s: %w", imageNameTag, platform, err)
					// 		CheckIfError(logger, newError, true)
					// 	}

					// 	PrintInfo(logger, "starting pushing %s docker image for %s platform", imageNameTag, platform)
					// 	err = DockerImagePush(cli, imageNameTag, platform, config, logger)
					// 	if err != nil {
					// 		newError := fmt.Errorf("failed to push docker image %s for platform %s: %w", imageNameTag, platform, err)
					// 		CheckIfError(logger, newError, true)
					// 	}
					// 	PrintInfo(logger, "docker image %s for platform %s builded and pushed successfully", imageNameTag, platform)
					// }
				}

				// rsync data on target_folder from git_sub_folder if specified
				if config.SYNC.DO_SUBFOLDER_SYNC {
					PrintInfo(logger, "start sync subfolder for tag %s", strMaxTag)
					err = Rsync(config.SYNC.TARGET_FOLDER, filepath.Join(config.GIT.LOCAL_GIT_FOLDER, config.SYNC.GIT_SUB_FOLDER))
					CheckIfError(logger, err, true)
					PrintInfo(logger, "successfully sync subfolder for tag %s", strMaxTag)
				}

				if config.DEPLOY.DO_MANIFEST_DEPLOY {
					// now it's time to get final manifest for k8s/k3s deployment from given template
					PrintInfo(logger, "start applying release %s", strMaxTag)
					manifestToApply, err := GenerateManifest(config.DEPLOY.MANIFESTS_K8S,
						struct {
							Release   string
							Image     string
							PgSecrets string
						}{
							Release:   strMaxTag,
							Image:     imageNameTag,
							PgSecrets: "/root/.config/pg",
						})
					if e := CheckIfError(logger, err, false); e != nil {
						PrintInfo(logger, "job %s is completed with error", config.COMMON.JOB_NAME)
					}

					// switch to given context
					_, err = RunExternalCmd("", fmt.Sprintf("error while switching to context %s", config.DEPLOY.CONTEXT_K8s),
						"kubectx", config.DEPLOY.CONTEXT_K8s)
					CheckIfError(logger, err, true)

					//  now apply it in given cluster
					// command := []string{"kubectl", "apply", "-f", "-", "--dry-run=client")}
					command := []string{"kubectl", "apply", "-f", "-"}
					outManifestApply, err := RunExternalCmd(manifestToApply, "error while applying manifest", command[0], command[1:]...)
					CheckIfError(logger, err, true)

					// waiting some time for changes take effect
					checkIntervals := GetIntervals(config.COMMON.CHECK_INTERVAL)
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
				currentSHA, err = CalculateSHA256(config.COMMON.JOB_PATH)
				if err != nil {
					CheckIfErrorFmt(logger, err, fmt.Errorf("calculate current SHA256 error: %w", err), false)
					PrintInfo(logger, "job %s completes", config.COMMON.JOB_NAME)
					return
				}

				if currentSHA != originalSHA {
					var newConfig *Config
					newConfig, err = getOneConfig(config.COMMON.JOB_PATH)
					if err != nil {
						CheckIfErrorFmt(logger, err, fmt.Errorf("read current config for job %s: %w", config.COMMON.JOB_NAME, err), false)
						PrintInfo(logger, "job %s completes", config.COMMON.JOB_NAME)
						return
					}
					config = newConfig
					if !config.COMMON.IS_ACTIVE {
						PrintInfo(logger, "job %s completes", config.COMMON.JOB_NAME)
						return
					}
					wg.Add(1)
					go RunOneJob(config, wg)
					PrintInfo(logger, "job %s now restarting", config.COMMON.JOB_NAME)
					return

				}

				time.Sleep(time.Duration(config.COMMON.CHECK_INTERVAL-totalWaitSeconds) * time.Second)
			}
		}
	}

	if !config.GIT.DO_GIT_CLONE && config.DEPLOY.DO_WATCH_IMAGE_TAG {
		// TODO: do variant of job execution then git repo do not cloned but
		// we watch for sha of docker image in deployment and compare with this one in cluster
		_ = !config.GIT.DO_GIT_CLONE
	}

}
