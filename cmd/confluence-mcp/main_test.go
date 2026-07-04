package main

import (
	"os"
	"testing"
)

func TestRun_MissingConfig(t *testing.T) {
	os.Clearenv()
	os.Args = []string{"confluence-mcp"}
	if err := run(); err == nil {
		t.Fatal("expected error with missing config, got nil")
	}
}
