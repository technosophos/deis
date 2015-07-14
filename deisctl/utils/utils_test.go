package utils

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestResolvePath ensures ResolvePath replaces $HOME and ~ as expected
func TestResolvePath(t *testing.T) {
	t.Parallel()

	paths := []string{"$HOME/foo/bar", "~/foo/bar"}
	expected := os.Getenv("HOME") + "/foo/bar"
	for _, path := range paths {
		resolved := ResolvePath(path)
		if resolved != expected {
			t.Errorf("Test failed: expected %s, got %s", expected, resolved)
		}
	}
}

func TestColorize(t *testing.T) {
	out := Colorize("{{.Red}}Hello {{.Default}}World{{.UnderGreen}}!{{.Default}}")
	expected := "\033[0;31mHello \033[0mWorld\033[4;32m!\033[0m"
	if out != expected {
		t.Errorf("Expected '%s', got '%s'", expected, out)
	}
}

func ExampleColorize() {
	out := Colorize("{{.Red}}Hello {{.Default}}World{{.UnderGreen}}!{{.Default}}")
	fmt.Println(out)
}

func TestDeisIfy(t *testing.T) {
	d := DeisIfy("Test")
	if strings.Contains(d, "Deis1") {
		t.Errorf("Failed to compile template")
	}
	if !strings.Contains(d, "Test") {
		t.Errorf("Failed to render template")
	}
}
