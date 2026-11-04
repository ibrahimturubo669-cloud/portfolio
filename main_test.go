package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvironmentFindsBackendEnvFromServerDirectory(t *testing.T) {
	root := t.TempDir()
	serverDir := filepath.Join(root, "backend", "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatal(err)
	}

	envPath := filepath.Join(root, "backend", ".env")
	if err := os.WriteFile(
		envPath,
		[]byte("KTFC_TEST_ENV_VALUE=loaded\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(serverDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	originalValue, hadValue := os.LookupEnv("KTFC_TEST_ENV_VALUE")
	if err := os.Unsetenv("KTFC_TEST_ENV_VALUE"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadValue {
			_ = os.Setenv("KTFC_TEST_ENV_VALUE", originalValue)
		} else {
			_ = os.Unsetenv("KTFC_TEST_ENV_VALUE")
		}
	})

	loaded, err := loadEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded {
		t.Fatal("loadEnvironment() reported no .env file")
	}
	if got, want := os.Getenv("KTFC_TEST_ENV_VALUE"), "loaded"; got != want {
		t.Errorf("environment value = %q, want %q", got, want)
	}
}
