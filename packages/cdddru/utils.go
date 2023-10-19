package cdddru

import (
	"bytes"
	"crypto/sha256"
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

func IsStringEmpty(s string) bool {
	return len(s) == 0
}

func IsStringNotEmpty(s string) bool {
	return !(len(s) == 0)
}

func CheckFolderPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func GetEnvVar(varName, defValue string) string {

	value := os.Getenv(varName)
	if len(value) == 0 {
		value = defValue
	}
	return value
}

// CheckArgs should be used to ensure the right command line arguments are
// passed before executing an example.
func CheckArgs(logger *Logger, isExit bool, arg ...string) {
	if len(os.Args) < len(arg)+1 && !IsStringNotEmpty(arg[1]) {
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

func RunExternalCmdsPiped(stdinStr, errorPrefix string, commands [][]string) (string, error) {
	if len(errorPrefix) == 0 {
		errorPrefix = fmt.Sprintf("error occured in %v commands", "pipe of")
	}
	if len(commands) < 2 {
		if err != nil {
			return "", fmt.Errorf("%v: %v ", errorPrefix, "at least two commands are required")
		}
	}
	var outBuf, errBuf bytes.Buffer
	// cmd.Stdout = &outBuf
	// cmd.Stderr = &errBuf

	var cmd []*exec.Cmd
	var err error

	// Create the command objects
	for _, c := range commands {
		cmd = append(cmd, exec.Command(c[0], c[1:]...))
	}

	// Connect the commands in a pipeline
	for i := 0; i < len(cmd)-1; i++ {
		currCmd := cmd[i]
		if len(stdinStr) > 0 && i == 0 {
			currCmd.Stdin = strings.NewReader(stdinStr)
		}
		nextCmd := cmd[i+1]

		pipe, err := currCmd.StdoutPipe()
		if err != nil {
			return "", fmt.Errorf("%v: error creating pipe: %w ", errorPrefix, err)
		}
		nextCmd.Stdin = pipe
	}

	// Set the last command's stdout to os.Stdout
	lastCmd := cmd[len(cmd)-1]
	lastCmd.Stdout = &outBuf
	lastCmd.Stderr = &errBuf

	// Start the commands in reverse order
	for i := len(cmd) - 1; i >= 0; i-- {
		err = cmd[i].Start()
		if err != nil {
			return "", fmt.Errorf("%v: error starting pipe: %w ", errorPrefix, err)
		}
	}

	// Wait for the commands to finish
	for _, c := range cmd {
		err = c.Wait()
		if err != nil {
			return "", fmt.Errorf("%v: %v < details: (%v) >", errorPrefix, err, errBuf.String())
		}
	}
	if len(errBuf.String()) > 0 {
		return "", fmt.Errorf("%v: %v < details: (%v) >", errorPrefix, err, errBuf.String())
	}
	return outBuf.String(), nil
}

func RunExternalCmd(stdinString, errorPrefix string, commandName string,
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

func ReplaceEnvs(content string) (string, error) {
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
		replacingText := GetEnvVar(match[1], "")
		contentString = strings.ReplaceAll(contentString, replacedText, replacingText)
	}
	return contentString, nil
}

func GetIntervals(configInterval int) [5]int {
	if configInterval < 120 {
		configInterval = 120
	}
	waitApplyingTimeSeconds := 3 * configInterval / 4
	intervalToWaitSeconds := waitApplyingTimeSeconds / 5
	checkIntervals := [5]int{intervalToWaitSeconds, intervalToWaitSeconds, intervalToWaitSeconds, intervalToWaitSeconds, intervalToWaitSeconds}
	if intervalToWaitSeconds > 120 {
		checkIntervals[0] = 120
		totalWait := checkIntervals[0]
		for i := 1; i < 4; i++ {
			restWait := waitApplyingTimeSeconds - totalWait
			newInterval := restWait / (5 - i)
			fmt.Println(i, restWait, (5 - i), newInterval, totalWait)
			if newInterval > checkIntervals[i-1]*2 {
				checkIntervals[i] = checkIntervals[i-1] * 2
			} else {
				checkIntervals[i] = newInterval
			}
			totalWait += checkIntervals[i]
		}
		checkIntervals[4] = waitApplyingTimeSeconds - totalWait
	}
	return checkIntervals
}

func TiifFunc(ifCase bool, funcTrue func(args ...interface{}) (interface{}, error),
	funcFalse func(args ...interface{}) (interface{}, error), args ...interface{}) (interface{}, error) {

	if ifCase {
		return funcTrue(args)
	}
	return funcFalse(args)
}

func Tiif(ifCase bool, vTrue interface{}, vFalse interface{}) interface{} {
	if ifCase {
		return vTrue
	}
	return vFalse
}

func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
