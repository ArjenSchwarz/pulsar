package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

const launchAgentLabel = "io.arjen.pulsar"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>serve</string>
        <string>--base-url</string>
        <string>{{.BaseURL}}</string>
        <string>--port</string>
        <string>{{.Port}}</string>
        <string>--dir</string>
        <string>{{.Dir}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>
`

type plistData struct {
	Label      string
	BinaryPath string
	BaseURL    string
	Port       string
	Dir        string
	LogPath    string
}

func newInstallCmd() *cobra.Command {
	var baseURLFlag string
	var portFlag int
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and load the pulsar LaunchAgent",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			baseURL := strings.TrimSpace(baseURLFlag)
			if baseURL == "" {
				baseURL = cfg.BaseURL
			}
			if baseURL == "" {
				baseURL = suggestBaseURL()
				if baseURL == "" {
					return fmt.Errorf("--base-url is required (no tailscale hostname detected)")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Using base URL from tailscale status: %s\n", baseURL)
			}
			port := portFlag
			if port == 0 {
				port = cfg.Port
				if port == 0 {
					port = 8765
				}
			}

			binary, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolving binary path: %w", err)
			}
			binary, _ = filepath.EvalSymlinks(binary)

			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			plistPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
			logPath := filepath.Join(home, "Library", "Logs", "pulsar.log")

			data := plistData{
				Label:      launchAgentLabel,
				BinaryPath: binary,
				BaseURL:    baseURL,
				Port:       fmt.Sprintf("%d", port),
				Dir:        cfg.Dir,
				LogPath:    logPath,
			}

			tmpl, err := template.New("plist").Parse(plistTemplate)
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(plistPath, buf.Bytes(), 0o644); err != nil {
				return fmt.Errorf("writing plist: %w", err)
			}

			// Reload if already loaded; otherwise just load.
			_ = exec.Command("launchctl", "unload", plistPath).Run()
			if out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
				return fmt.Errorf("launchctl load: %w: %s", err, out)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed LaunchAgent at %s\n", plistPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Logs: %s\n", logPath)
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "Next: expose it on your tailnet with:")
			fmt.Fprintf(cmd.OutOrStdout(), "  tailscale serve --bg --https=443 http://127.0.0.1:%d\n", port)
			fmt.Fprintln(cmd.OutOrStdout(), "Verify with: tailscale serve status")
			return nil
		},
	}
	cmd.Flags().StringVar(&baseURLFlag, "base-url", "", "public base URL (defaults to tailscale-derived value if available)")
	cmd.Flags().IntVar(&portFlag, "port", 0, "HTTP port to bind on 127.0.0.1 (default 8765)")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Unload and remove the pulsar LaunchAgent",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			plistPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
			if _, err := os.Stat(plistPath); os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to do — plist not present.")
				return nil
			}
			if out, err := exec.Command("launchctl", "unload", plistPath).CombinedOutput(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "launchctl unload: %s\n", out)
			}
			if err := os.Remove(plistPath); err != nil {
				return fmt.Errorf("removing plist: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", plistPath)
			return nil
		},
	}
}

// suggestBaseURL queries `tailscale status --json` for the device's tailnet FQDN.
// Returns "" if tailscale is not installed or the hostname can't be determined.
func suggestBaseURL() string {
	out, err := exec.Command("tailscale", "status", "--self=true", "--peers=false").Output()
	if err != nil {
		return ""
	}
	// The first non-empty field of the first line is the device's tailnet name.
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		host := fields[1]
		if strings.Contains(host, ".ts.net") {
			return "https://" + host
		}
	}
	return ""
}
