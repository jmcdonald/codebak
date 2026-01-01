package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := NewForTesting(&out, &errOut, []string{"codebak", "version"})
	c.Version = "1.2.3"
	c.Run()

	output := out.String()
	if !strings.Contains(output, "codebak v1.2.3") {
		t.Errorf("version output = %q, expected to contain 'codebak v1.2.3'", output)
	}
}

func TestHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := NewForTesting(&out, &errOut, []string{"codebak", "help"})
	c.Run()

	output := out.String()
	if !strings.Contains(output, "Incremental Code Backup Tool") {
		t.Errorf("help output = %q, expected to contain usage info", output)
	}
	if !strings.Contains(output, "codebak run") {
		t.Errorf("help output = %q, expected to contain 'codebak run'", output)
	}
}

func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	exitCalled := false
	exitCode := 0

	c := NewForTesting(&out, &errOut, []string{"codebak", "unknown-cmd"})
	c.Exit = func(code int) {
		exitCalled = true
		exitCode = code
	}
	c.Run()

	errOutput := errOut.String()
	if !strings.Contains(errOutput, "Unknown command: unknown-cmd") {
		t.Errorf("error output = %q, expected to contain 'Unknown command'", errOutput)
	}
	if !exitCalled {
		t.Error("Exit should have been called")
	}
	if exitCode != 1 {
		t.Errorf("exit code = %d, expected 1", exitCode)
	}
}

func TestNoCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := NewForTesting(&out, &errOut, []string{"codebak"})
	c.Run()

	output := out.String()
	if !strings.Contains(output, "No command specified") {
		t.Errorf("output = %q, expected to contain 'No command specified'", output)
	}
}

func TestVerifyMissingProject(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	exitCalled := false

	c := NewForTesting(&out, &errOut, []string{"codebak", "verify"})
	c.Exit = func(code int) {
		exitCalled = true
	}
	c.Run()

	output := out.String()
	if !strings.Contains(output, "Usage: codebak verify") {
		t.Errorf("output = %q, expected usage message", output)
	}
	if !exitCalled {
		t.Error("Exit should have been called")
	}
}

func TestRecoverMissingProject(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	exitCalled := false

	c := NewForTesting(&out, &errOut, []string{"codebak", "recover"})
	c.Exit = func(code int) {
		exitCalled = true
	}
	c.Run()

	output := out.String()
	if !strings.Contains(output, "Usage: codebak recover") {
		t.Errorf("output = %q, expected usage message", output)
	}
	if !exitCalled {
		t.Error("Exit should have been called")
	}
}

func TestListMissingProject(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	exitCalled := false

	c := NewForTesting(&out, &errOut, []string{"codebak", "list"})
	c.Exit = func(code int) {
		exitCalled = true
	}
	c.Run()

	output := out.String()
	if !strings.Contains(output, "Usage: codebak list") {
		t.Errorf("output = %q, expected usage message", output)
	}
	if !exitCalled {
		t.Error("Exit should have been called")
	}
}

func TestPrintUsage(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := NewForTesting(&out, &errOut, []string{"codebak"})
	c.PrintUsage()

	output := out.String()

	// Check for key sections
	expectedPhrases := []string{
		"codebak - Incremental Code Backup Tool",
		"codebak run",
		"codebak list",
		"codebak verify",
		"codebak recover",
		"codebak install",
		"codebak uninstall",
		"codebak status",
		"codebak init",
		"codebak version",
		"codebak help",
		"~/.codebak/config.yaml",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("usage output missing expected phrase: %q", phrase)
		}
	}
}

func TestCLINew(t *testing.T) {
	c := New("1.0.0")

	if c.Out == nil {
		t.Error("Out should not be nil")
	}
	if c.Err == nil {
		t.Error("Err should not be nil")
	}
	if c.Version != "1.0.0" {
		t.Errorf("Version = %q, expected '1.0.0'", c.Version)
	}
	if c.Exit == nil {
		t.Error("Exit should not be nil")
	}
	if c.green == nil || c.yellow == nil || c.cyan == nil || c.gray == nil || c.red == nil {
		t.Error("color functions should not be nil")
	}
}
