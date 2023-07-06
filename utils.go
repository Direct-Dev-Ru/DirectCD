package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// ParseFlags parses the command line args, allowing flags to be
// specified after positional args.
func ParseFlags() error {
	return ParseFlagSet(flag.CommandLine, os.Args[1:])
}

// ParseFlagSet works like flagset.Parse(), except positional arguments are not
// required to come after flag arguments.
func ParseFlagSet(flagset *flag.FlagSet, args []string) error {
	var positionalArgs []string
	for {
		if err := flagset.Parse(args); err != nil {
			return err
		}
		// Consume all the flags that were parsed as flags.
		args = args[len(args)-flagset.NArg():]
		if len(args) == 0 {
			break
		}
		// There's at least one flag remaining and it must be a positional arg since
		// we consumed all args that were parsed as flags. Consume just the first
		// one, and retry parsing, since subsequent args may be flags.
		positionalArgs = append(positionalArgs, args[0])
		args = args[1:]
	}
	// Parse just the positional args so that flagset.Args()/flagset.NArgs()
	// return the expected value.
	// Note: This should never return an error.
	return flagset.Parse(positionalArgs)
}

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
func CheckArgs(logger *Logger, isExit bool, arg ...string) {
	if len(os.Args) < len(arg)+1 && !isStringNotEmpty(arg[1]) {
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

func replaceEnvs(content string) (string, error) {
	contentString := strings.TrimSpace(content)
	pattern := `{{\$(.*?)}}`
	// Compile the regular expression
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return content, err
	}
	// Find all matches in the text
	matches := regex.FindAllStringSubmatch(contentString, -1)

	// iterate through matches
	for _, match := range matches {
		replacedText := match[0]
		replacingText := getEnvVar(match[1], "")
		contentString = strings.ReplaceAll(contentString, replacedText, replacingText)
	}
	return contentString, nil
}
