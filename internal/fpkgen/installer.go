package fpkgen

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const fallbackInstallVolume = "1"
const installVolumeLabel = "watchcow.install_volume"

// Installer handles fnOS application installation via appcenter-cli
type Installer struct {
	appcenterCLIPath string
}

// NewInstaller creates a new installer
func NewInstaller() (*Installer, error) {
	// Find appcenter-cli
	cliPath, err := findAppcenterCLI()
	if err != nil {
		return nil, err
	}

	return &Installer{
		appcenterCLIPath: cliPath,
	}, nil
}

// findAppcenterCLI locates the appcenter-cli binary
func findAppcenterCLI() (string, error) {
	// Try common locations on fnOS
	paths := []string{
		"/var/apps/appcenter/target/bin/appcenter-cli",
		"/usr/bin/appcenter-cli",
		"/usr/local/bin/appcenter-cli",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			slog.Debug("Found appcenter-cli", "path", p)
			return p, nil
		}
	}

	// Try PATH
	path, err := exec.LookPath("appcenter-cli")
	if err == nil {
		slog.Debug("Found appcenter-cli in PATH", "path", path)
		return path, nil
	}

	// Try using 'which' command as fallback
	whichCmd := exec.Command("which", "appcenter-cli")
	output, err := whichCmd.Output()
	if err == nil {
		p := strings.TrimSpace(string(output))
		if p != "" {
			slog.Debug("Found appcenter-cli via which", "path", p)
			return p, nil
		}
	}

	return "", fmt.Errorf("appcenter-cli not found in common locations or PATH")
}

// InstallLocal installs an application from local directory
func (i *Installer) InstallLocal(appDir string, labels map[string]string) error {
	slog.Info("Installing fnOS app via appcenter-cli", "appDir", appDir)

	volume := i.resolveInstallVolume(labels)
	args := []string{"install-local", "--volume", volume}

	cmd := exec.Command(i.appcenterCLIPath, args...)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("appcenter-cli install-local failed: %w", err)
	}

	slog.Info("Successfully installed fnOS app")
	return nil
}

func (i *Installer) resolveInstallVolume(labels map[string]string) string {
	if volume := installVolumeFromLabels(labels); volume != "" {
		slog.Info("Using install volume from label", "label", installVolumeLabel, "volume", volume)
		return volume
	}

	if volume := i.defaultInstallVolume(); volume != "" {
		slog.Info("Using default install volume from appcenter-cli", "volume", volume)
		return volume
	}

	slog.Info("No install volume configured, using fallback volume", "volume", fallbackInstallVolume)
	return fallbackInstallVolume
}

func installVolumeFromLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	return parseInstallVolume(labels[installVolumeLabel])
}

func (i *Installer) defaultInstallVolume() string {
	cmd := exec.Command(i.appcenterCLIPath, "default-volume")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("Failed to check default install volume", "error", err, "output", strings.TrimSpace(string(output)))
		return ""
	}

	return parseInstallVolume(string(output))
}

func parseInstallVolume(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	volumeIndex, err := strconv.Atoi(trimmed)
	if err != nil || volumeIndex <= 0 {
		return ""
	}

	return strconv.Itoa(volumeIndex)
}

// Uninstall uninstalls an application
func (i *Installer) Uninstall(appName string) error {
	slog.Info("Uninstalling fnOS app", "appName", appName)

	// First stop the app
	stopCmd := exec.Command(i.appcenterCLIPath, "stop", appName)
	stopCmd.Run() // Ignore stop errors

	// Try to uninstall with appName as argument
	uninstallCmd := exec.Command(i.appcenterCLIPath, "uninstall", appName)
	output, err := uninstallCmd.CombinedOutput()
	if err != nil {
		// Try without argument (some versions may work differently)
		slog.Debug("Uninstall with appName failed, trying alternate method",
			"appName", appName,
			"output", string(output))

		// Log warning but don't fail - app may need manual uninstall
		slog.Warn("Could not uninstall fnOS app automatically",
			"appName", appName,
			"hint", "may need manual uninstall from App Center")
		return nil
	}

	slog.Info("Successfully uninstalled fnOS app", "appName", appName)
	return nil
}

// StartApp starts an installed application
func (i *Installer) StartApp(appName string) error {
	slog.Info("Starting fnOS app", "appName", appName)

	cmd := exec.Command(i.appcenterCLIPath, "start", appName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	return nil
}

// StopApp stops an installed application
func (i *Installer) StopApp(appName string) error {
	slog.Info("Stopping fnOS app", "appName", appName)

	cmd := exec.Command(i.appcenterCLIPath, "stop", appName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop app: %w", err)
	}

	return nil
}

// IsAppInstalled checks if an app is installed by parsing appcenter-cli list output
func (i *Installer) IsAppInstalled(appName string) bool {
	cmd := exec.Command(i.appcenterCLIPath, "list")
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("Failed to list apps", "error", err)
		return false
	}

	// Parse table output - look for appName in the first column
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Skip header and separator lines
		if strings.HasPrefix(line, "│") {
			// Extract first column (app name)
			parts := strings.Split(line, "│")
			if len(parts) >= 2 {
				installedApp := strings.TrimSpace(parts[1])
				if installedApp == appName {
					slog.Debug("App already installed", "appName", appName)
					return true
				}
			}
		}
	}

	return false
}
