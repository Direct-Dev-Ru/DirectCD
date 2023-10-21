package cdddru

import (
	"os"
	"testing"
)

func TestSetAuth(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	t.Log(tempDir)
	// Set up environment variables for testing
	os.Setenv("DOCKER_SERVER", "example.com")
	os.Setenv("DOCKER_TOKEN", "myToken")
	os.Setenv("DOCKER_USER", "myUsername")

	dockerConfig := DockerConfig{}

	// Test when dockerconfig directory exists
	err := os.MkdirAll(tempDir+"/dockerconfig", 0700)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempDir+"/dockerconfig/config.json", []byte(`{}`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = dockerConfig.SetAuth(tempDir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test when dockerconfig directory does not exist
	err = os.RemoveAll(tempDir + "/dockerconfig")
	if err != nil {
		t.Fatal(err)
	}

	err = dockerConfig.SetAuth(tempDir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Clean up environment variables
	os.Unsetenv("DOCKER_SERVER")
	os.Unsetenv("DOCKER_TOKEN")
	os.Unsetenv("DOCKER_USER")
}
