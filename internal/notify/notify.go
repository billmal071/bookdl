package notify

import (
	"os/exec"
	"runtime"

	"github.com/billmal071/bookdl/internal/config"
)

// Notification types
const (
	TypeSuccess = "success"
	TypeError   = "error"
	TypeInfo    = "info"
)

// Send sends a desktop notification if enabled in config
func Send(title, message, notifyType string) {
	if !config.Get().Downloads.Notifications {
		return
	}

	// Send notification in background
	go sendNotification(title, message, notifyType)
}

// DownloadComplete sends a download complete notification
func DownloadComplete(filename string) {
	Send("Download Complete", filename, TypeSuccess)
}

// DownloadFailed sends a download failed notification
func DownloadFailed(filename, reason string) {
	msg := filename
	if reason != "" {
		msg += ": " + reason
	}
	Send("Download Failed", msg, TypeError)
}

// QueueComplete sends a queue completion notification
func QueueComplete(completed, failed int) {
	var msg string
	if failed == 0 {
		msg = "All downloads completed successfully"
	} else {
		msg = "Completed with some failures"
	}
	Send("Queue Complete", msg, TypeInfo)
}

func sendNotification(title, message, notifyType string) {
	switch runtime.GOOS {
	case "linux":
		sendLinuxNotification(title, message, notifyType)
	case "darwin":
		sendMacNotification(title, message)
	case "windows":
		sendWindowsNotification(title, message)
	}
}

func sendLinuxNotification(title, message, notifyType string) {
	// Try notify-send (most common on Linux)
	icon := "dialog-information"
	switch notifyType {
	case TypeSuccess:
		icon = "dialog-ok"
	case TypeError:
		icon = "dialog-error"
	}

	cmd := exec.Command("notify-send", "-i", icon, "-a", "bookdl", title, message)
	cmd.Run()
}

func sendMacNotification(title, message string) {
	script := `display notification "` + escapeAppleScript(message) + `" with title "` + escapeAppleScript(title) + `"`
	cmd := exec.Command("osascript", "-e", script)
	cmd.Run()
}

func sendWindowsNotification(title, message string) {
	// Use PowerShell for Windows notifications
	script := `
	[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
	[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
	$template = '<toast><visual><binding template="ToastText02"><text id="1">` + escapeXML(title) + `</text><text id="2">` + escapeXML(message) + `</text></binding></visual></toast>'
	$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
	$xml.LoadXml($template)
	$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
	[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("bookdl").Show($toast)
	`
	cmd := exec.Command("powershell", "-Command", script)
	cmd.Run()
}

func escapeAppleScript(s string) string {
	// Escape backslashes and double quotes for AppleScript
	result := ""
	for _, c := range s {
		if c == '\\' || c == '"' {
			result += "\\"
		}
		result += string(c)
	}
	return result
}

func escapeXML(s string) string {
	// Escape XML special characters
	result := ""
	for _, c := range s {
		switch c {
		case '<':
			result += "&lt;"
		case '>':
			result += "&gt;"
		case '&':
			result += "&amp;"
		case '"':
			result += "&quot;"
		case '\'':
			result += "&apos;"
		default:
			result += string(c)
		}
	}
	return result
}
