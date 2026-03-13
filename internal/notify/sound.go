package notify

import (
	"os/exec"
	"runtime"

	"github.com/victor-falcon/falcode/internal/config"
)

// SoundEvent identifies which agent state transition triggered a sound.
type SoundEvent int

const (
	// SoundEventIdle is fired when the agent finishes a task (becomes idle).
	SoundEventIdle SoundEvent = iota
	// SoundEventPermission is fired when the agent requests a permission decision.
	SoundEventPermission
)

// PlaySound emits a notification sound for the given event, subject to the
// notifications config. It spawns an async goroutine to play the platform
// system sound:
//   - macOS: afplay /System/Library/Sounds/Glass.aiff  (idle)
//     afplay /System/Library/Sounds/Funk.aiff   (permission)
//   - Linux: paplay (freedesktop event sounds, if available)
//
// Fire-and-forget; errors are silently discarded so that a missing sound
// daemon never breaks the UI.
func PlaySound(event SoundEvent, notif *config.NotificationsConfig) {
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
		// sound event names. Errors are silently discarded.
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
