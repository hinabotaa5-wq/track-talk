package config

import (
	"strings"
	"testing"
)

func TestValidateMeetVoice(t *testing.T) {
	for _, tc := range []struct {
		name, url, key, secret string
		enabled, valid         bool
	}{
		{name: "disabled", valid: true},
		{"cloud", "wss://voice.livekit.cloud", "key", "secret", true, true},
		{"self-hosted", "wss://voice.example.com", "key", "secret", true, false},
		{"trailing slash", "wss://voice.livekit.cloud/", "key", "secret", true, true},
		{"missing key", "wss://voice.livekit.cloud", "", "secret", true, false},
		{"missing secret", "wss://voice.livekit.cloud", "key", "", true, false},
		{"insecure", "ws://voice.livekit.cloud", "key", "secret", true, false},
		{"http", "https://voice.livekit.cloud", "key", "secret", true, false},
		{"credentials", "wss://private:credential@voice.livekit.cloud", "key", "secret", true, false},
		{"path", "wss://voice.livekit.cloud/path", "key", "secret", true, false},
		{"query", "wss://voice.livekit.cloud?key=private", "key", "secret", true, false},
		{"fragment", "wss://voice.livekit.cloud/#private", "key", "secret", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Config{MeetVoiceEnabled: tc.enabled, LiveKitURL: tc.url, LiveKitAPIKey: tc.key, LiveKitAPISecret: tc.secret}).ValidateMeetVoice()
			if (err == nil) != tc.valid {
				t.Fatalf("validation: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("configuration leaked")
			}
		})
	}
}
