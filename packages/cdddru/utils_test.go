package cdddru

import (
	"os"
	"testing"
)

func TestRunExternalCmd(t *testing.T) {
	stdinString := ""
	errorPrefix := "test error prefix:"
	commandName := "kubectl"
	commandArgs := []string{"get", "nodes"}
	os.Setenv("KUBECONFIG", "/run/configs/kubeconfig/config")
	output, err := RunExternalCmd(stdinString, errorPrefix, commandName, commandArgs...)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	t.Log(output)

	// expectedOutput := "arg1 arg2\n"
	// if output != expectedOutput {
	// 	t.Errorf("Expected output %v, got %v", expectedOutput, output)
	// }
}
