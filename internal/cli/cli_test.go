package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmcdonald/codebak/internal/backup"
	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/manifest"
	"github.com/jmcdonald/codebak/internal/recovery"
)

// ============================================================================
// Mock implementations for testing
// ============================================================================

// mockConfigService implements ConfigService for testing.
type mockConfigService struct {
	config        *config.Config
	loadErr       error
	saveErr       error
	configPath    string
	configPathErr error
	defaultCfgErr error
}

func newMockConfigService() *mockConfigService {
	return &mockConfigService{
		config: &config.Config{
			SourceDir: "/test/source",
			BackupDir: "/test/backup",
		},
		configPath: "/test/.codebak/config.yaml",
	}
}

func (m *mockConfigService) Load() (*config.Config, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.config, nil
}

func (m *mockConfigService) Save(cfg *config.Config) error {
	return m.saveErr
}

func (m *mockConfigService) ConfigPath() (string, error) {
	if m.configPathErr != nil {
		return "", m.configPathErr
	}
	return m.configPath, nil
}

func (m *mockConfigService) DefaultConfig() (*config.Config, error) {
	if m.defaultCfgErr != nil {
		return nil, m.defaultCfgErr
	}
	return m.config, nil
}

// mockBackupService implements BackupService for testing.
type mockBackupService struct {
	backupResults    []backup.BackupResult
	singleResult     backup.BackupResult
	runBackupErr     error
	backupProjectErr error
}

func newMockBackupService() *mockBackupService {
	return &mockBackupService{}
}

func (m *mockBackupService) BackupProject(cfg *config.Config, project string) backup.BackupResult {
	if m.backupProjectErr != nil {
		return backup.BackupResult{Project: project, Error: m.backupProjectErr}
	}
	m.singleResult.Project = project
	return m.singleResult
}

func (m *mockBackupService) RunBackup(cfg *config.Config) ([]backup.BackupResult, error) {
	if m.runBackupErr != nil {
		return nil, m.runBackupErr
	}
	return m.backupResults, nil
}

// mockRecoveryService implements RecoveryService for testing.
type mockRecoveryService struct {
	verifyErr      error
	recoverErr     error
	listVersions   []manifest.BackupEntry
	listVersionErr error
	lastRecoverOpts recovery.RecoverOptions
}

func newMockRecoveryService() *mockRecoveryService {
	return &mockRecoveryService{}
}

func (m *mockRecoveryService) Verify(cfg *config.Config, project, version string) error {
	return m.verifyErr
}

func (m *mockRecoveryService) Recover(cfg *config.Config, opts recovery.RecoverOptions) error {
	m.lastRecoverOpts = opts
	return m.recoverErr
}

func (m *mockRecoveryService) ListVersions(cfg *config.Config, project string) ([]manifest.BackupEntry, error) {
	if m.listVersionErr != nil {
		return nil, m.listVersionErr
	}
	return m.listVersions, nil
}

// mockLaunchdService implements LaunchdService for testing.
type mockLaunchdService struct {
	installed   bool
	installErr  error
	uninstallErr error
	statusLoaded bool
	statusErr   error
	plistPath   string
	logPath     string
}

func newMockLaunchdService() *mockLaunchdService {
	return &mockLaunchdService{
		plistPath: "/test/Library/LaunchAgents/com.user.codebak.plist",
		logPath:   "/test/.codebak/codebak.log",
	}
}

func (m *mockLaunchdService) IsInstalled() bool {
	return m.installed
}

func (m *mockLaunchdService) Install(hour, minute int) error {
	if m.installErr != nil {
		return m.installErr
	}
	m.installed = true
	return nil
}

func (m *mockLaunchdService) Uninstall() error {
	if m.uninstallErr != nil {
		return m.uninstallErr
	}
	m.installed = false
	return nil
}

func (m *mockLaunchdService) Status() (bool, error) {
	return m.statusLoaded, m.statusErr
}

func (m *mockLaunchdService) PlistPath() string {
	return m.plistPath
}

func (m *mockLaunchdService) LogPath() string {
	return m.logPath
}

// ============================================================================
// Test helper
// ============================================================================

// testCLI creates a CLI for testing with mocks and exit tracking.
type testCLI struct {
	*CLI
	out        *bytes.Buffer
	errOut     *bytes.Buffer
	exitCode   int
	exitCalled bool
}

func newTestCLI(args []string) *testCLI {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	tc := &testCLI{
		out:    out,
		errOut: errOut,
	}

	noColor := func(a ...interface{}) string { return strings.Trim(strings.Join(toStrings(a), " "), " ") }

	tc.CLI = &CLI{
		Out:     out,
		Err:     errOut,
		Version: "test",
		Args:    args,
		Exit: func(code int) {
			tc.exitCode = code
			tc.exitCalled = true
		},
		green:  noColor,
		yellow: noColor,
		cyan:   noColor,
		gray:   noColor,
		red:    noColor,
	}

	return tc
}

func toStrings(a []interface{}) []string {
	result := make([]string, len(a))
	for i, v := range a {
		if s, ok := v.(string); ok {
			result[i] = s
		} else {
			result[i] = ""
		}
	}
	return result
}

// ============================================================================
// Tests
// ============================================================================

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

func TestVersionFlags(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"version command", "version"},
		{"-v flag", "-v"},
		{"--version flag", "--version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestCLI([]string{"codebak", tt.arg})
			tc.Version = "2.0.0"
			tc.Run()

			if !strings.Contains(tc.out.String(), "codebak v2.0.0") {
				t.Errorf("expected version output, got %q", tc.out.String())
			}
		})
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

func TestHelpFlags(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"help command", "help"},
		{"-h flag", "-h"},
		{"--help flag", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestCLI([]string{"codebak", tt.arg})
			tc.Run()

			if !strings.Contains(tc.out.String(), "codebak - Incremental Code Backup Tool") {
				t.Errorf("expected help output, got %q", tc.out.String())
			}
		})
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
		"codebak --version",
		"codebak --help",
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

// ============================================================================
// InitConfig tests
// ============================================================================

func TestInitConfigSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "init"})
	mockCfg := newMockConfigService()
	tc.ConfigSvc = mockCfg

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called, exitCode=%d", tc.exitCode)
	}
	if !strings.Contains(tc.out.String(), "Created config at") {
		t.Errorf("expected success message, got %q", tc.out.String())
	}
	if !strings.Contains(tc.out.String(), mockCfg.configPath) {
		t.Errorf("expected config path in output, got %q", tc.out.String())
	}
}

func TestInitConfigSaveError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "init"})
	mockCfg := newMockConfigService()
	mockCfg.saveErr = errors.New("disk full")
	tc.ConfigSvc = mockCfg

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1), got exitCalled=%v, exitCode=%d", tc.exitCalled, tc.exitCode)
	}
	if !strings.Contains(tc.errOut.String(), "Error saving config") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// RunBackup tests
// ============================================================================

func TestRunBackupSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "run"})
	mockCfg := newMockConfigService()
	mockBackup := newMockBackupService()
	mockBackup.backupResults = []backup.BackupResult{
		{Project: "project1", Size: 1024, FileCount: 10, Reason: "git HEAD changed"},
		{Project: "project2", Skipped: true, Reason: "no changes"},
	}
	tc.ConfigSvc = mockCfg
	tc.BackupSvc = mockBackup

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	output := tc.out.String()
	if !strings.Contains(output, "Scanning") {
		t.Errorf("expected scanning message, got %q", output)
	}
	if !strings.Contains(output, "project1") {
		t.Errorf("expected project1 in output, got %q", output)
	}
	if !strings.Contains(output, "project2") {
		t.Errorf("expected project2 in output, got %q", output)
	}
	if !strings.Contains(output, "1 backed up") {
		t.Errorf("expected backed up count, got %q", output)
	}
	if !strings.Contains(output, "1 skipped") {
		t.Errorf("expected skipped count, got %q", output)
	}
}

func TestRunBackupSingleProject(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "run", "myproject"})
	mockCfg := newMockConfigService()
	mockBackup := newMockBackupService()
	mockBackup.singleResult = backup.BackupResult{
		Size: 2048, FileCount: 20, Reason: "files modified",
	}
	tc.ConfigSvc = mockCfg
	tc.BackupSvc = mockBackup

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	output := tc.out.String()
	if !strings.Contains(output, "myproject") {
		t.Errorf("expected myproject in output, got %q", output)
	}
}

func TestRunBackupConfigLoadError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "run"})
	mockCfg := newMockConfigService()
	mockCfg.loadErr = errors.New("config not found")
	tc.ConfigSvc = mockCfg

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error loading config") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

func TestRunBackupRunBackupError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "run"})
	mockCfg := newMockConfigService()
	mockBackup := newMockBackupService()
	mockBackup.runBackupErr = errors.New("source dir not found")
	tc.ConfigSvc = mockCfg
	tc.BackupSvc = mockBackup

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error:") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

func TestRunBackupWithErrors(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "run"})
	mockCfg := newMockConfigService()
	mockBackup := newMockBackupService()
	mockBackup.backupResults = []backup.BackupResult{
		{Project: "project1", Error: errors.New("backup failed")},
		{Project: "project2", Size: 1024, FileCount: 5, Reason: "success"},
	}
	tc.ConfigSvc = mockCfg
	tc.BackupSvc = mockBackup

	tc.Run()

	output := tc.out.String()
	if !strings.Contains(output, "project1") {
		t.Errorf("expected project1 in output, got %q", output)
	}
	if !strings.Contains(output, "backup failed") {
		t.Errorf("expected error message in output, got %q", output)
	}
	if !strings.Contains(output, "1 errors") {
		t.Errorf("expected error count, got %q", output)
	}
}

// ============================================================================
// InstallLaunchd tests
// ============================================================================

func TestInstallLaunchdSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "install"})
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = false
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	output := tc.out.String()
	if !strings.Contains(output, "Installed launchd schedule") {
		t.Errorf("expected success message, got %q", output)
	}
	if !strings.Contains(output, "daily at 3:00 AM") {
		t.Errorf("expected schedule info, got %q", output)
	}
	if !strings.Contains(output, mockLaunchd.plistPath) {
		t.Errorf("expected plist path, got %q", output)
	}
	if !strings.Contains(output, mockLaunchd.logPath) {
		t.Errorf("expected log path, got %q", output)
	}
}

func TestInstallLaunchdAlreadyInstalled(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "install"})
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = true
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.out.String(), "already installed") {
		t.Errorf("expected already installed message, got %q", tc.out.String())
	}
}

func TestInstallLaunchdError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "install"})
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = false
	mockLaunchd.installErr = errors.New("permission denied")
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error installing launchd") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// UninstallLaunchd tests
// ============================================================================

func TestUninstallLaunchdSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "uninstall"})
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = true
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !strings.Contains(tc.out.String(), "Uninstalled launchd schedule") {
		t.Errorf("expected success message, got %q", tc.out.String())
	}
}

func TestUninstallLaunchdNotInstalled(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "uninstall"})
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = false
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.out.String(), "launchd not installed") {
		t.Errorf("expected not installed message, got %q", tc.out.String())
	}
}

func TestUninstallLaunchdError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "uninstall"})
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = true
	mockLaunchd.uninstallErr = errors.New("plist locked")
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error uninstalling launchd") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// ShowStatus tests
// ============================================================================

func TestShowStatusSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "status"})
	mockCfg := newMockConfigService()
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = false
	tc.ConfigSvc = mockCfg
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	output := tc.out.String()
	if !strings.Contains(output, "codebak status:") {
		t.Errorf("expected status header, got %q", output)
	}
	if !strings.Contains(output, mockCfg.config.SourceDir) {
		t.Errorf("expected source dir, got %q", output)
	}
	if !strings.Contains(output, mockCfg.config.BackupDir) {
		t.Errorf("expected backup dir, got %q", output)
	}
	if !strings.Contains(output, "not installed") {
		t.Errorf("expected not installed status, got %q", output)
	}
}

func TestShowStatusLaunchdInstalled(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "status"})
	mockCfg := newMockConfigService()
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = true
	mockLaunchd.statusLoaded = true
	tc.ConfigSvc = mockCfg
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	output := tc.out.String()
	if !strings.Contains(output, "installed & loaded") {
		t.Errorf("expected installed & loaded status, got %q", output)
	}
}

func TestShowStatusLaunchdInstalledNotLoaded(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "status"})
	mockCfg := newMockConfigService()
	mockLaunchd := newMockLaunchdService()
	mockLaunchd.installed = true
	mockLaunchd.statusLoaded = false
	tc.ConfigSvc = mockCfg
	tc.LaunchdSvc = mockLaunchd

	tc.Run()

	output := tc.out.String()
	if !strings.Contains(output, "installed (not loaded)") {
		t.Errorf("expected installed (not loaded) status, got %q", output)
	}
}

func TestShowStatusConfigLoadError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "status"})
	mockCfg := newMockConfigService()
	mockCfg.loadErr = errors.New("config corrupted")
	tc.ConfigSvc = mockCfg

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error loading config") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// RunVerify tests
// ============================================================================

func TestRunVerifySuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "verify", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !strings.Contains(tc.out.String(), "Checksum verified for myproject") {
		t.Errorf("expected success message, got %q", tc.out.String())
	}
}

func TestRunVerifyWithVersion(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "verify", "myproject", "20240101-120000"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !strings.Contains(tc.out.String(), "Checksum verified") {
		t.Errorf("expected success message, got %q", tc.out.String())
	}
}

func TestRunVerifyConfigLoadError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "verify", "myproject"})
	mockCfg := newMockConfigService()
	mockCfg.loadErr = errors.New("config error")
	tc.ConfigSvc = mockCfg

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error loading config") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

func TestRunVerifyVerificationFailed(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "verify", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.verifyErr = errors.New("checksum mismatch")
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Verification failed") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// RunRecover tests
// ============================================================================

func TestRunRecoverSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	output := tc.out.String()
	if !strings.Contains(output, "Recovering myproject") {
		t.Errorf("expected recovering message, got %q", output)
	}
	if !strings.Contains(output, "Successfully recovered myproject") {
		t.Errorf("expected success message, got %q", output)
	}
}

func TestRunRecoverWithWipe(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject", "--wipe"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !mockRecovery.lastRecoverOpts.Wipe {
		t.Error("expected Wipe option to be set")
	}
	if !strings.Contains(tc.out.String(), "wiping current") {
		t.Errorf("expected wipe message, got %q", tc.out.String())
	}
}

func TestRunRecoverWithArchive(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject", "--archive"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !mockRecovery.lastRecoverOpts.Archive {
		t.Error("expected Archive option to be set")
	}
	if !strings.Contains(tc.out.String(), "archiving current") {
		t.Errorf("expected archive message, got %q", tc.out.String())
	}
}

func TestRunRecoverWithVersion(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject", "--version=20240101-120000"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if mockRecovery.lastRecoverOpts.Version != "20240101-120000" {
		t.Errorf("expected version 20240101-120000, got %q", mockRecovery.lastRecoverOpts.Version)
	}
}

func TestRunRecoverWipeAndArchive(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject", "--wipe", "--archive"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.out.String(), "Cannot use both --wipe and --archive") {
		t.Errorf("expected error message, got %q", tc.out.String())
	}
}

func TestRunRecoverConfigLoadError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject"})
	mockCfg := newMockConfigService()
	mockCfg.loadErr = errors.New("config error")
	tc.ConfigSvc = mockCfg

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error loading config") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

func TestRunRecoverRecoveryFailed(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.recoverErr = errors.New("project already exists")
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Recovery failed") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// ListBackups tests
// ============================================================================

func TestListBackupsSuccess(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "list", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.listVersions = []manifest.BackupEntry{
		{
			File:      "20240101-120000.zip",
			SizeBytes: 1024,
			FileCount: 10,
			GitHead:   "abc1234567890",
			CreatedAt: time.Now(),
		},
		{
			File:      "20240102-120000.zip",
			SizeBytes: 2048,
			FileCount: 20,
			GitHead:   "",
			CreatedAt: time.Now(),
		},
	}
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	output := tc.out.String()
	if !strings.Contains(output, "Backups for myproject") {
		t.Errorf("expected header, got %q", output)
	}
	if !strings.Contains(output, "20240101-120000") {
		t.Errorf("expected version, got %q", output)
	}
	if !strings.Contains(output, "abc1234") {
		t.Errorf("expected truncated git head, got %q", output)
	}
}

func TestListBackupsEmpty(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "list", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.listVersions = []manifest.BackupEntry{}
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !strings.Contains(tc.out.String(), "No backups found for myproject") {
		t.Errorf("expected no backups message, got %q", tc.out.String())
	}
}

func TestListBackupsConfigLoadError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "list", "myproject"})
	mockCfg := newMockConfigService()
	mockCfg.loadErr = errors.New("config error")
	tc.ConfigSvc = mockCfg

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error loading config") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

func TestListBackupsListVersionsError(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "list", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.listVersionErr = errors.New("project not found")
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1)")
	}
	if !strings.Contains(tc.errOut.String(), "Error:") {
		t.Errorf("expected error message, got %q", tc.errOut.String())
	}
}

func TestListBackupsShortGitHead(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "list", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.listVersions = []manifest.BackupEntry{
		{
			File:      "20240101-120000.zip",
			SizeBytes: 1024,
			FileCount: 10,
			GitHead:   "abc12",  // Less than 7 chars
			CreatedAt: time.Now(),
		},
	}
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	// Short git head should be displayed as-is
	if !strings.Contains(tc.out.String(), "abc12") {
		t.Errorf("expected short git head, got %q", tc.out.String())
	}
}

// ============================================================================
// Default service fallback tests
// ============================================================================

func TestDefaultServiceFallbacks(t *testing.T) {
	// Test that when no services are injected, the CLI creates default services
	tc := newTestCLI([]string{"codebak", "version"})

	// These should not panic - they create default services
	cfgSvc := tc.configSvc()
	if cfgSvc == nil {
		t.Error("configSvc should not be nil")
	}

	backupSvc := tc.backupSvc()
	if backupSvc == nil {
		t.Error("backupSvc should not be nil")
	}

	recoverySvc := tc.recoverySvc()
	if recoverySvc == nil {
		t.Error("recoverySvc should not be nil")
	}

	launchdSvc := tc.launchdSvc()
	if launchdSvc == nil {
		t.Error("launchdSvc should not be nil")
	}
}

// ============================================================================
// Run command dispatch tests
// ============================================================================

func TestRunCommandDispatch(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string // expected substring in output
	}{
		{"run command", []string{"codebak", "run"}, "Scanning"},
		{"init command", []string{"codebak", "init"}, "Created config at"},
		{"install command", []string{"codebak", "install"}, "Installed launchd"},
		{"uninstall command", []string{"codebak", "uninstall"}, "Uninstalled launchd"},
		{"status command", []string{"codebak", "status"}, "codebak status:"},
		{"verify command", []string{"codebak", "verify", "project"}, "Checksum verified"},
		{"recover command", []string{"codebak", "recover", "project"}, "Successfully recovered"},
		{"list command", []string{"codebak", "list", "project"}, "No backups found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestCLI(tt.args)

			// Set up mocks for all services
			mockCfg := newMockConfigService()
			mockBackup := newMockBackupService()
			mockRecovery := newMockRecoveryService()
			mockLaunchd := newMockLaunchdService()

			// Configure mocks for success scenarios
			mockBackup.backupResults = []backup.BackupResult{}
			mockLaunchd.installed = strings.Contains(tt.name, "uninstall")

			tc.ConfigSvc = mockCfg
			tc.BackupSvc = mockBackup
			tc.RecoverySvc = mockRecovery
			tc.LaunchdSvc = mockLaunchd

			tc.Run()

			output := tc.out.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("expected output to contain %q, got %q", tt.expected, output)
			}
		})
	}
}

// ============================================================================
// NewForTesting tests
// ============================================================================

func TestNewForTesting(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := NewForTesting(&out, &errOut, []string{"codebak", "test"})

	if c.Out != &out {
		t.Error("Out should be set to provided buffer")
	}
	if c.Err != &errOut {
		t.Error("Err should be set to provided buffer")
	}
	if c.Version != "test" {
		t.Errorf("Version = %q, expected 'test'", c.Version)
	}
	if len(c.Args) != 2 {
		t.Errorf("Args length = %d, expected 2", len(c.Args))
	}
	if c.Exit == nil {
		t.Error("Exit should not be nil")
	}

	// Test color functions return plain text
	if c.green("test") != "test" {
		t.Error("green should return plain text")
	}
	if c.yellow("test") != "test" {
		t.Error("yellow should return plain text")
	}
	if c.cyan("test") != "test" {
		t.Error("cyan should return plain text")
	}
	if c.gray("test") != "test" {
		t.Error("gray should return plain text")
	}
	if c.red("test") != "test" {
		t.Error("red should return plain text")
	}

	// Test that Exit function is callable (it stores the code internally)
	c.Exit(42)
	// No panic means success - the exit code is stored internally
}

// ============================================================================
// Service injection tests
// ============================================================================

func TestServiceInjectionPriority(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "version"})

	// Test that injected services take priority over defaults
	mockCfg := newMockConfigService()
	mockCfg.configPath = "/custom/path/config.yaml"
	tc.ConfigSvc = mockCfg

	// The configSvc() should return our mock, not the default
	svc := tc.configSvc()
	path, err := svc.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	if path != "/custom/path/config.yaml" {
		t.Errorf("expected custom path, got %q", path)
	}

	// Test backup service injection
	mockBackup := newMockBackupService()
	tc.BackupSvc = mockBackup
	bSvc := tc.backupSvc()
	if bSvc != mockBackup {
		t.Error("expected injected backup service")
	}

	// Test recovery service injection
	mockRecovery := newMockRecoveryService()
	tc.RecoverySvc = mockRecovery
	rSvc := tc.recoverySvc()
	if rSvc != mockRecovery {
		t.Error("expected injected recovery service")
	}

	// Test launchd service injection
	mockLaunchd := newMockLaunchdService()
	tc.LaunchdSvc = mockLaunchd
	lSvc := tc.launchdSvc()
	if lSvc != mockLaunchd {
		t.Error("expected injected launchd service")
	}
}

// ============================================================================
// Color function edge cases
// ============================================================================

func TestColorFunctionsWithMultipleArgs(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "version"})

	// Test that color functions handle multiple arguments
	result := tc.green("hello", "world")
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}

	result = tc.cyan("a", "b", "c")
	if result != "a b c" {
		t.Errorf("expected 'a b c', got %q", result)
	}
}

// ============================================================================
// Additional edge case tests
// ============================================================================

func TestRunBackupWithAllResultTypes(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "run"})
	mockCfg := newMockConfigService()
	mockBackup := newMockBackupService()
	// Mix of all result types: success, skipped, error
	mockBackup.backupResults = []backup.BackupResult{
		{Project: "success-project", Size: 1024, FileCount: 10, Reason: "git changed"},
		{Project: "skipped-project", Skipped: true, Reason: "no changes"},
		{Project: "error-project", Error: errors.New("disk full")},
	}
	tc.ConfigSvc = mockCfg
	tc.BackupSvc = mockBackup

	tc.Run()

	output := tc.out.String()
	// All projects should appear
	if !strings.Contains(output, "success-project") {
		t.Errorf("expected success-project in output")
	}
	if !strings.Contains(output, "skipped-project") {
		t.Errorf("expected skipped-project in output")
	}
	if !strings.Contains(output, "error-project") {
		t.Errorf("expected error-project in output")
	}
	// Summary should be correct
	if !strings.Contains(output, "1 backed up") {
		t.Errorf("expected '1 backed up' in output")
	}
	if !strings.Contains(output, "1 skipped") {
		t.Errorf("expected '1 skipped' in output")
	}
	if !strings.Contains(output, "1 errors") {
		t.Errorf("expected '1 errors' in output")
	}
}

func TestRunRecoverWithAllFlags(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "recover", "myproject", "--wipe", "--version=20240101-120000"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	if !mockRecovery.lastRecoverOpts.Wipe {
		t.Error("expected Wipe option to be set")
	}
	if mockRecovery.lastRecoverOpts.Version != "20240101-120000" {
		t.Errorf("expected version, got %q", mockRecovery.lastRecoverOpts.Version)
	}
}

func TestListBackupsWithExactly7CharGitHead(t *testing.T) {
	tc := newTestCLI([]string{"codebak", "list", "myproject"})
	mockCfg := newMockConfigService()
	mockRecovery := newMockRecoveryService()
	mockRecovery.listVersions = []manifest.BackupEntry{
		{
			File:      "20240101-120000.zip",
			SizeBytes: 1024,
			FileCount: 10,
			GitHead:   "abc1234", // Exactly 7 chars
			CreatedAt: time.Now(),
		},
	}
	tc.ConfigSvc = mockCfg
	tc.RecoverySvc = mockRecovery

	tc.Run()

	if tc.exitCalled {
		t.Errorf("Exit should not have been called")
	}
	// Exactly 7 char git head should be displayed as-is (no truncation)
	if !strings.Contains(tc.out.String(), "abc1234") {
		t.Errorf("expected git head, got %q", tc.out.String())
	}
}

// ============================================================================
// Helper function tests
// ============================================================================

func TestToStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []string
	}{
		{
			name:     "string values",
			input:    []interface{}{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "non-string values",
			input:    []interface{}{123, 45.6, true},
			expected: []string{"", "", ""},
		},
		{
			name:     "mixed values",
			input:    []interface{}{"hello", 42, "world"},
			expected: []string{"hello", "", "world"},
		},
		{
			name:     "empty slice",
			input:    []interface{}{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, expected %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: got %q, expected %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// ============================================================================
// UI command tests (for completeness - though skipped in Run())
// ============================================================================

func TestRunDispatchesUICommand(t *testing.T) {
	// The "ui" command isn't in the dispatch - let's verify unknown command handling
	tc := newTestCLI([]string{"codebak", "ui"})

	tc.Run()

	// "ui" is not a recognized command, so it should print unknown command
	if !tc.exitCalled || tc.exitCode != 1 {
		t.Errorf("expected Exit(1) for unknown command 'ui'")
	}
	if !strings.Contains(tc.errOut.String(), "Unknown command: ui") {
		t.Errorf("expected unknown command message, got %q", tc.errOut.String())
	}
}

// ============================================================================
// Integration test helpers
// ============================================================================

func TestMockServicesImplementInterfaces(t *testing.T) {
	// Compile-time checks that mocks implement the interfaces
	var _ ConfigService = newMockConfigService()
	var _ BackupService = newMockBackupService()
	var _ RecoveryService = newMockRecoveryService()
	var _ LaunchdService = newMockLaunchdService()
}

// ============================================================================
// Default service wrapper tests (integration-style - calls real packages)
// These test the thin wrapper layer only when safe to do so
// ============================================================================

func TestDefaultConfigServiceConfigPath(t *testing.T) {
	// This method doesn't touch the filesystem, safe to call
	svc := &defaultConfigService{}
	path, err := svc.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	if path == "" {
		t.Error("ConfigPath should return a non-empty path")
	}
	if !strings.Contains(path, ".codebak") {
		t.Errorf("ConfigPath should contain '.codebak', got %q", path)
	}
}

func TestDefaultConfigServiceDefaultConfig(t *testing.T) {
	// This method doesn't touch the filesystem, safe to call
	svc := &defaultConfigService{}
	cfg, err := svc.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("DefaultConfig should return a non-nil config")
	}
	if cfg.SourceDir == "" {
		t.Error("DefaultConfig should have a SourceDir")
	}
	if cfg.BackupDir == "" {
		t.Error("DefaultConfig should have a BackupDir")
	}
}

func TestDefaultLaunchdServicePaths(t *testing.T) {
	// These methods don't touch the filesystem, safe to call
	svc := &defaultLaunchdService{}

	plistPath := svc.PlistPath()
	if plistPath == "" {
		t.Error("PlistPath should return a non-empty path")
	}
	if !strings.Contains(plistPath, "codebak") {
		t.Errorf("PlistPath should contain 'codebak', got %q", plistPath)
	}

	logPath := svc.LogPath()
	if logPath == "" {
		t.Error("LogPath should return a non-empty path")
	}
	if !strings.Contains(logPath, "codebak") {
		t.Errorf("LogPath should contain 'codebak', got %q", logPath)
	}
}

func TestDefaultLaunchdServiceIsInstalled(t *testing.T) {
	// IsInstalled just checks if a file exists - safe to call
	// It will return false on most test systems where the plist isn't installed
	svc := &defaultLaunchdService{}
	_ = svc.IsInstalled() // Just ensure it doesn't panic
}

func TestDefaultLaunchdServiceStatus(t *testing.T) {
	// Status is safe to call, it just runs launchctl list
	svc := &defaultLaunchdService{}
	_, _ = svc.Status() // Just ensure it doesn't panic
}
