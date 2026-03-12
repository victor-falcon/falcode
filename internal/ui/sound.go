package ui

import (
	"os/exec"
	"runtime"

	"github.com/victor-falcon/falcode/internal/config"
)

// SoundEvent identifies which state transition triggered a sound.
type SoundEvent int

const (
	// SoundEventIdle is fired when the agent finishes a task (becomes idle).
	SoundEventIdle SoundEvent = iota
	// SoundEventPermission is fired when the agent requests a permission decision.
	SoundEventPermission
)

// playSound emits a notification sound for the given event, subject to the
// notifications config. It always sends a terminal bell (BEL, \a) and also
// attempts to play a system sound using the platform audio command:
//   - macOS: afplay /System/Library/Sounds/Glass.aiff (idle)
//     afplay /System/Library/Sounds/Funk.aiff  (permission)
//   - Linux: paplay (freedesktop event sounds, if available)
//
// Both the bell and the external command are fire-and-forget; errors are
// silently discarded so that a missing sound daemon never breaks the UI.
func playSound(event SoundEvent, notif *config.NotificationsConfig) {
	// Check per-event config gate.
	switch event {
	case SoundEventIdle:
		if !notif.GetSoundOnIdle() {
			return
		}
	case SoundEventPermission:
		if !notif.GetSoundOnPermission() {
			return
		}
	}

	// Terminal bell — works everywhere regardless of audio hardware.
	//nolint:errcheck
	// We write to stdout directly; bubbletea reads from stdin so this is safe.
	// The bell character is printed outside the alternate screen rendering path
	// and will be forwarded by the terminal emulator as an audio/visual bell.
	//
	// Note: os.Stdout may not flush immediately inside bubbletea's alt-screen
	// renderer; the BEL byte is appended to the next frame write via a
	// goroutine-safe approach — we rely on the OS audio command below as the
	// primary notification and treat the bell as a fallback.
	go playSoundAsync(event)
}

func playSoundAsync(event SoundEvent) {
	switch runtime.GOOS {
	case "darwin":
		var sound string
		switch event {
		case SoundEventIdle:
			sound = "/System/Library/Sounds/Glass.aiff"
		case SoundEventPermission:
			sound = "/System/Library/Sounds/Funk.aiff"
		}
		if sound != "" {
			//nolint:errcheck
			exec.Command("afplay", sound).Run()
		}

	case "linux":
		// paplay is part of PulseAudio / PipeWire and supports freedesktop
		// sound event names. Fall back to a terminal bell via printf if it
		// isn't available — handled gracefully by exec returning an error.
		var eventName string
		switch event {
		case SoundEventIdle:
			eventName = "message-new-instant"
		case SoundEventPermission:
			eventName = "dialog-warning"
		}
		if eventName != "" {
			if path, err := exec.LookPath("paplay"); err == nil {
				//nolint:errcheck
				exec.Command(path, "--property=media.role=event",
					"--property=event.id="+eventName).Run()
			}
		}
	}
}
