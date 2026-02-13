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
	cfg := config.Get()
	if !cfg.Downloads.Notifications {
		return
	}

	// Send notification in background
	go sendNotification(title, message, notifyType)

	// Play sound if enabled
	if cfg.Downloads.SoundEnabled {
		go playSound(notifyType)
	}
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
	// Ignore errors - notification tools may not be installed
	_ = cmd.Run()
}

func sendMacNotification(title, message string) {
	script := `display notification "` + escapeAppleScript(message) + `" with title "` + escapeAppleScript(title) + `"`
	cmd := exec.Command("osascript", "-e", script)
	// Ignore errors - osascript should always be available on macOS
	_ = cmd.Run()
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
	// Ignore errors - toast notifications may not be available on all Windows versions
	_ = cmd.Run()
}

// playSound plays a notification sound based on the platform
func playSound(notifyType string) {
	switch runtime.GOOS {
	case "linux":
		playLinuxSound(notifyType)
	case "darwin":
		playMacSound(notifyType)
	case "windows":
		playWindowsSound(notifyType)
	}
}

// playLinuxSound plays a sound on Linux using paplay (PulseAudio) or aplay (ALSA)
func playLinuxSound(notifyType string) {
	// Map notification types to system sounds
	soundName := "message"
	switch notifyType {
	case TypeSuccess:
		soundName = "complete"
	case TypeError:
		soundName = "dialog-error"
	case TypeInfo:
		soundName = "message"
	}

	// Try paplay first (PulseAudio - most common)
	cmd := exec.Command("paplay", "/usr/share/sounds/freedesktop/stereo/"+soundName+".oga")
	if err := cmd.Run(); err != nil {
		// Fallback to canberra-gtk-play
		cmd = exec.Command("canberra-gtk-play", "-i", soundName)
		_ = cmd.Run()
	}
}

// playMacSound plays a sound on macOS
func playMacSound(notifyType string) {
	// Map notification types to macOS system sounds
	soundName := "Ping"
	switch notifyType {
	case TypeSuccess:
		soundName = "Glass"
	case TypeError:
		soundName = "Basso"
	case TypeInfo:
		soundName = "Ping"
	}

	cmd := exec.Command("afplay", "/System/Library/Sounds/"+soundName+".aiff")
	_ = cmd.Run()
}

// playWindowsSound plays a sound on Windows
func playWindowsSound(notifyType string) {
	// Map notification types to Windows system sounds
	soundAlias := "SystemNotification"
	switch notifyType {
	case TypeSuccess:
		soundAlias = "SystemNotification"
	case TypeError:
		soundAlias = "SystemHand"
	case TypeInfo:
		soundAlias = "SystemAsterisk"
	}

	// Use rundll32 to play system sounds
	cmd := exec.Command("rundll32", "user32.dll,MessageBeep", soundAlias)
	_ = cmd.Run()
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
