package cdddru

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
)

func GetDeploymentReadinessStatus(config Config, imageNameTag string) (bool, error) {
	// kubectl get deployment main-site  -n test-app | grep main-site | awk '{print $2}'
	pipeCommands := [][]string{
		{"kubectx", config.CONTEXT_K8s},
		{"kubectl", "get", "deployment", config.DEPLOYMENT_NAME_K8s, "-n", "test-app", "-o", "wide"},
		{"grep", config.DEPLOYMENT_NAME_K8s + ".*" + imageNameTag},
		{"awk", `{print $2}`},
	}
	out, err := RunExternalCmdsPiped("", "pipe error", pipeCommands)
	if err != nil || len(out) == 0 {
		return false, nil
	}
	replicasSlice := strings.Split(out, "/")
	if len(replicasSlice) != 2 {
		return false, nil
	}
	return strings.TrimSpace(replicasSlice[0]) == strings.TrimSpace(replicasSlice[1]), nil
}

// get current image tag from k8s deployment - we will run kubectl ...
func GetImageTag(cfg Config) (string, error) {

	var deployment, namespace, dockerImage = cfg.DEPLOYMENT_NAME_K8s, cfg.NAMESPACE_K8s, cfg.DOCKER_IMAGE

	_, err := RunExternalCmd("", "error while switching to context "+cfg.CONTEXT_K8s, "kubectx", cfg.CONTEXT_K8s)
	if err != nil {
		return "", fmt.Errorf("failed to switch context: %v (%v)", cfg.CONTEXT_K8s, err)
	}
	// Create a buffer for stderr
	var errThread bytes.Buffer

	// Build the kubectl command
	cmd := exec.Command("kubectl", "get", "deployment", deployment, "-n", namespace, "-o", "jsonpath={.spec.template.spec.containers}")
	// Run the kubectl command and capture the output
	cmd.Stderr = &errThread
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute request to kubernetes: %v (%v)", err, errThread.String())
	}

	// Trim leading/trailing spaces and newlines from the output
	outputString := strings.TrimSpace(string(output))

	// Define the regular expression pattern
	pattern := dockerImage + `:` + `(` + cfg.GIT_TAG_PREFIX + `?\d{1,2}\.\d{1,2}\.\d{1,2})`
	// Compile the regular expression
	regex := regexp.MustCompile(pattern)

	// Find the matches in the larger string
	matches := regex.FindStringSubmatch(outputString)

	// Check if a match was found
	if len(matches) > 0 {
		return matches[1], nil
	}

	return "v0.0.0", nil
}

// ----------------- //

// Convert a version tag to a comparable numeric value
func ConvertTagToNumeric(tag, prefix string) (int64, error) {
	tag = strings.TrimPrefix(tag, prefix) // Remove the leading prefix
	parts := strings.Split(tag, ".")      // Split the tag into major, minor, and patch parts

	// Convert parts to integers
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to convert major version to integer: %v", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("failed to convert minor version to integer: %v", err)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("failed to convert patch version to integer: %v", err)
	}

	// Combine parts into a single numeric value max version subpart is 99
	numeric := int64(major)*10000 + int64(minor)*100 + int64(patch)
	return numeric, nil
}

func GetCommitHashByTag(gitRepository *git.Repository, tag string) (string, error) {
	refTag, err := gitRepository.Tag(tag)
	if err != nil {
		return "", err
	}
	tagObj, err := gitRepository.TagObject(refTag.Hash())
	if err != nil {
		return "", err
	}
	// Resolve the commit hash from the tag reference
	commitHash := tagObj.Target
	return commitHash.String(), nil
	// PrintInfo(logger, "Tag '%s' has commit hash %s", "v1.0.11", commitHash.String())
}

func CheckOutByCommitHash(gitRepository *git.Repository, commitHash string) error {
	// Resolve the commit object from the hash
	hash := plumbing.NewHash(commitHash)
	commitObj, err := gitRepository.CommitObject(hash)
	if err != nil {
		return err
	}
	// Checkout the commit
	wt, err := gitRepository.Worktree()
	if err != nil {
		return err
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash: commitObj.Hash,
	})
	if err != nil {
		return err
	}
	files, err := GetWorktreeFiles(wt)
	if err != nil {
		return err
	}
	for _, file := range files {
		fmt.Println(file.Name())
	}
	return nil
}

func GetWorktreeFiles(wt *git.Worktree) ([]fs.FileInfo, error) {

	fileList, err := wt.Filesystem.ReadDir(".")
	return fileList, err
}

func GetTagsFromGitRepo(gitRepository *git.Repository, tagPrefix string) ([]string, error) {

	var repoTags []string
	repoTags = make([]string, 0, 4)

	tagRefs, err := gitRepository.Tags()
	if err != nil {
		return nil, err
	}
	// Iterate over the tags and print their names
	tagRefs.ForEach(func(tagRef *plumbing.Reference) error {
		if strings.HasPrefix(tagRef.Name().Short(), tagPrefix) {
			repoTags = append(repoTags, tagRef.Name().Short())
		}
		return nil
	})
	return repoTags, nil
}

func GetMaxTag(tags []string, maxTagValue string, prefix string) (int64, string, error) {
	var tagString string = "v1.0.0"
	if IsStringEmpty(maxTagValue) {
		maxTagValue = "v99.99.99"
	}
	nMaxTag, _ := ConvertTagToNumeric(tagString, prefix)
	nMaxTagValue, _ := ConvertTagToNumeric(maxTagValue, prefix)
	for _, tag := range tags {
		currentNumTag, err := ConvertTagToNumeric(tag, prefix)
		if err != nil {
			return -1, "", err
		}
		if currentNumTag > nMaxTag && currentNumTag <= nMaxTagValue {
			nMaxTag = currentNumTag
			tagString = tag
		}
	}
	return nMaxTag, tagString, nil
}

func CompareTwoTags(tag1, tag2, prefix string) (int, error) {

	nTag1, err := ConvertTagToNumeric(tag1, prefix)
	if err != nil {
		return -2, err
	}
	nTag2, err := ConvertTagToNumeric(tag2, prefix)
	if err != nil {
		return -3, err
	}

	switch {
	case nTag1 == nTag2:
		return 0, nil
	case nTag1 > nTag2:
		return -1, nil
	case nTag1 < nTag2:
		return 1, nil
	default:
		return -10, nil
	}

}

func Rsync(targetPath, folderPath string) error {
	// Perform the rsync operation using the 'rsync' command
	var errThread bytes.Buffer
	err := os.MkdirAll(targetPath, os.ModePerm)
	if err != nil {
		return err
	}
	keys := Tiif(bool(FbVerbose), "-avzhq", "-avzh")
	cmd := exec.Command("rsync", keys.(string), "--delete", "--recursive", folderPath, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &errThread
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("rsync command failed: %v (%v)", err, errThread.String())
	}
	return nil
}

func GenerateManifest(templatePath string, data interface{}) (string, error) {
	// Create a new template and parse the template string
	rawManifest, err := os.ReadFile(templatePath)
	if err != nil {
		return "", err
	}
	manifest := string(rawManifest)
	manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, `"{{`, `{{`), `}}"`, `}}`)
	manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, `" {{`, `{{`), `}} "`, `}}`)
	manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, `' {{`, `{{`), `}} '`, `}}`)
	manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, `'{{`, `{{`), `}}'`, `}}`)

	tmplManifest, err := template.New("myTemplate").Parse(manifest)
	if err != nil {
		err = fmt.Errorf("error parsing template: %w", err)
		return "", err
	}
	var resultString string
	buf := &bytes.Buffer{}
	err = tmplManifest.Execute(buf, data)
	if err != nil {
		err = fmt.Errorf("error executing template: %w", err)
		return "", err

	}
	resultString = buf.String()
	return resultString, nil
}
