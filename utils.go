package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func isStringEmpty(s string) bool {
	return len(s) == 0
}
func isStringNotEmpty(s string) bool {
	return !(len(s) == 0)
}

func checkFolderPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func getEnvVar(varName, defValue string) string {

	value := os.Getenv(varName)
	if len(value) == 0 {
		value = defValue
	}
	return value
}

// CheckArgs should be used to ensure the right command line arguments are
// passed before executing an example.
func CheckArgs(isExit bool, arg ...string) {
	if len(os.Args) < len(arg)+1 {
		PrintWarning(logger, "Usage: %s %s", os.Args[0], strings.Join(arg, " "))
		if isExit {
			os.Exit(1)
		}
	}
}

func PrettyJsonEncode(data interface{}, out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "    ")
	if err := enc.Encode(data); err != nil {
		return err
	}
	return nil
}

func PrettyJsonEncodeToString(data interface{}) (string, error) {

	var buffer bytes.Buffer
	err := PrettyJsonEncode(data, &buffer)

	return buffer.String(), err
}

func runExternalCmd(stdinString, errorPrefix string, commandName string,
	commandArgs ...string) (string, error) {
	// Apply the Kubernetes manifest using the 'kubectl' command
	cmd := exec.Command(commandName, commandArgs...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if len(stdinString) > 0 {
		cmd.Stdin = strings.NewReader(stdinString)
	}
	err := cmd.Run()
	if len(errorPrefix) == 0 {
		errorPrefix = fmt.Sprintf("error occured in %v command", commandName)
	}
	if err != nil {
		return "", fmt.Errorf("%v: %v < details: (%v) >", errorPrefix, err, errBuf.String())
	}
	return outBuf.String(), nil
}
