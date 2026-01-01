package ports

// LaunchdService abstracts macOS launchd operations for testability.
// Production code uses MacLaunchdService adapter; tests use MockLaunchdService.
type LaunchdService interface {
	// PlistPath returns the path where the plist file should be stored.
	PlistPath() string

	// LogPath returns the path where logs should be written.
	LogPath() string

	// Install creates the plist file and loads the service.
	Install(execPath, configPath string, hour, minute int) error

	// Uninstall unloads the service and removes the plist file.
	Uninstall() error

	// IsInstalled checks if the service is currently installed.
	IsInstalled() bool

	// Status returns the current status of the service.
	// Returns "running", "loaded", "not loaded", or error message.
	Status() string
}
