package platform

import (
	"fmt"
	"os/exec"
)

// Notify sends a desktop notification using OS-native tools.
// urgency is one of "low", "normal", "critical".
func Notify(title, body, urgency string) error {
	switch DetectOS() {
	case PlatformMacOS:
		return notifyMacOS(title, body)
	case PlatformWSL:
		return notifyWSL(title, body)
	default:
		return notifyLinux(title, body, urgency)
	}
}

func notifyLinux(title, body, urgency string) error {
	cmd := exec.Command("notify-send", "--urgency", urgency, title, body)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("notify-send: %w (is libnotify-bin installed?)", err)
	}
	return nil
}

func notifyMacOS(title, body string) error {
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript: %w", err)
	}
	return nil
}

func notifyWSL(title, body string) error {
	// Use PowerShell through WSL interop
	ps := fmt.Sprintf(
		`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null; `+
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); `+
			`$textNodes = $template.GetElementsByTagName('text'); `+
			`$textNodes.Item(0).AppendChild($template.CreateTextNode('%s')) > $null; `+
			`$textNodes.Item(1).AppendChild($template.CreateTextNode('%s')) > $null; `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Hitch').Show($toast)`,
		escapePS(title), escapePS(body),
	)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Run(); err != nil {
		// Fall back to simple BurntToast or basic notification
		return notifyWSLFallback(title, body)
	}
	return nil
}

func notifyWSLFallback(title, body string) error {
	// Try simpler PowerShell approach
	ps := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; `+
			`[System.Windows.Forms.MessageBox]::Show('%s', '%s', 'OK', 'Information')`,
		escapePS(body), escapePS(title),
	)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("powershell: %w", err)
	}
	return nil
}

func escapePS(s string) string {
	// Basic PowerShell string escaping for single-quoted context
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
