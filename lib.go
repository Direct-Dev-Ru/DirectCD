package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
)

// get current image tag from k8s deployment - we will run kubectl ...
func getImageTag(cfg Config) (string, error) {

	var deployment, namespace, dockerImage = cfg.DEPLOYMENT_NAME_K8s, cfg.NAMESPACE_K8s, cfg.DOCKER_IMAGE

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

	// outputString = testOutput

	// Define the regular expression pattern
	pattern := dockerImage + `:` + `(` + cfg.GIT_TAG_PREFIX + `?\d{1,2}\.\d{1,2}\.\d{1,2})`
	// Compile the regular expression
	regex := regexp.MustCompile(pattern)

	// Find the matches in the larger string
	matches := regex.FindStringSubmatch(outputString)

	// Check if a match was found
	if len(matches) > 0 {
		// fmt.Println("Match found:", matches[1]) // index 1 for the captured group
		return matches[1], nil
	}

	// return "", fmt.Errorf("failed to match image tag in given deployment: %v", "no match found")
	return "v0.0.0", nil
}

// ----------------- //

// Convert a version tag to a comparable numeric value
func convertTagToNumeric(tag, prefix string) (int64, error) {
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

func getCommitHashByTag(gitRepository *git.Repository, tag string) (string, error) {
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

func getTagsFromGitRepo(gitRepository *git.Repository, tagPrefix string) ([]string, error) {

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

func getMaxTag(tags []string, maxTagValue string, prefix string) (int64, string, error) {
	var tagString string = "v1.0.0"
	if isStringEmpty(maxTagValue) {
		maxTagValue = "v99.99.99"
	}
	nMaxTag, _ := convertTagToNumeric(tagString, prefix)
	nMaxTagValue, _ := convertTagToNumeric(maxTagValue, prefix)
	for _, tag := range tags {
		currentNumTag, err := convertTagToNumeric(tag, prefix)
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

func compareTwoTags(tag1, tag2, prefix string) (int, error) {

	nTag1, err := convertTagToNumeric(tag1, prefix)
	if err != nil {
		return -2, err
	}
	nTag2, err := convertTagToNumeric(tag2, prefix)
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

func rsync(targetPath, folderPath string) error {
	// Perform the rsync operation using the 'rsync' command
	var errThread bytes.Buffer
	err := os.MkdirAll(targetPath, os.ModePerm)
	if err != nil {
		return err
	}

	cmd := exec.Command("rsync", "-avzh", "--delete", "--recursive", folderPath, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &errThread
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("rsync command failed: %v (%v)", err, errThread.String())
	}
	return nil
}

func generateManifest(templatePath string, data interface{}) (string, error) {
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
