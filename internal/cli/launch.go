package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/profiles"
	"github.com/spf13/cobra"
)

func newLaunchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launch [-- claude-flags...]",
		Short: "Apply a profile and launch Claude Code",
		Long: `Checks that the hitch proxy is running, applies the requested profile to
settings.local.json, then execs claude with full stdin/stdout/stderr pass-through.

Everything after -- is passed verbatim to claude.`,
		RunE: runLaunch,
	}
	cmd.Flags().String("profile", "", "Apply this profile before launching")
	cmd.Flags().String("cwd", "", "Change to this directory before launching")
	cmd.Flags().Bool("dry-run", false, "Print what would happen without executing")
	cmd.Flags().Bool("no-proxy-check", false, "Skip proxy health check")
	return cmd
}

func runLaunch(cmd *cobra.Command, args []string) error {
	profileName, _ := cmd.Flags().GetString("profile")
	cwdFlag, _ := cmd.Flags().GetString("cwd")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	noProxyCheck, _ := cmd.Flags().GetBool("no-proxy-check")

	// 1. Resolve CWD.
	if cwdFlag != "" {
		if _, err := os.Stat(cwdFlag); err != nil {
			return fmt.Errorf("--cwd %q: %w", cwdFlag, err)
		}
		if err := os.Chdir(cwdFlag); err != nil {
			return fmt.Errorf("chdir to %q: %w", cwdFlag, err)
		}
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// 2. Check proxy.
	proxyRunning := false
	if !noProxyCheck {
		proxyRunning = isProxyHealthy()
		if !proxyRunning {
			fmt.Fprintln(os.Stderr, "Warning: hitch proxy is not running. Start it with: ht proxy start")
			fmt.Fprintln(os.Stderr, "Continuing without proxy...")
		}
	}

	// 3. Apply profile.
	var p *profiles.Profile
	if profileName != "" {
		p, err = profiles.Load(profileName)
		if err != nil {
			return fmt.Errorf("loading profile %q: %w", profileName, err)
		}
		if !dryRun {
			if _, err := profiles.ApplyProfile(p, projectDir); err != nil {
				return fmt.Errorf("applying profile %q: %w", profileName, err)
			}
		}
	}

	// 4. Print dry-run and exit.
	if dryRun {
		printLaunchDryRun(profileName, projectDir, p, proxyRunning, args)
		return nil
	}

	// 5. Exec.
	db, _, dbErr := openDB()
	if dbErr == nil {
		_ = db.EventLog(state.Event{
			HookEvent:   "launch",
			ActionTaken: launchAction(profileName),
		})
		db.Close()
	}

	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH: %w", err)
	}

	argv := append([]string{claudeBin}, args...)
	return syscall.Exec(claudeBin, argv, os.Environ())
}

// isProxyHealthy returns true if the proxy health endpoint responds with 200.
func isProxyHealthy() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:9800/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func launchAction(profileName string) string {
	if profileName != "" {
		return "launch:profile=" + profileName
	}
	return "launch"
}

func printLaunchDryRun(profileName, projectDir string, p *profiles.Profile, proxyRunning bool, claudeArgs []string) {
	if profileName == "" {
		profileName = "(none)"
	}
	fmt.Printf("Profile: %s\n", profileName)
	fmt.Printf("Target:  %s/.claude/settings.local.json\n", projectDir)
	fmt.Println()

	if proxyRunning {
		fmt.Println("Proxy: running (http://localhost:9800)")
	} else {
		fmt.Println("Proxy: not running")
	}
	fmt.Println()

	if p != nil {
		if len(p.Env) > 0 {
			fmt.Println("Env block changes:")
			for k, v := range p.Env {
				fmt.Printf("  + %s = %s\n", k, v)
			}
			fmt.Println()
		}
		if len(p.Settings) > 0 {
			fmt.Println("Settings changes:")
			for k, v := range p.Settings {
				fmt.Printf("  + %s = %v\n", k, v)
			}
			fmt.Println()
		}
	}

	cmdLine := "claude"
	for _, a := range claudeArgs {
		cmdLine += " " + a
	}
	fmt.Printf("Command that would run:\n  %s\n", cmdLine)
	fmt.Println()
	fmt.Println("(dry-run: not launching)")
}
