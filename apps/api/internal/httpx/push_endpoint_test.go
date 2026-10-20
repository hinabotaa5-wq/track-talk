package httpx

import "testing"

func TestPushServiceHost(t *testing.T) {
	for host, want := range map[string]bool{
		"fcm.googleapis.com":                true,
		"updates.push.services.mozilla.com": true,
		"web.push.apple.com":                true,
		"api.push.apple.com":                true,
		"wns2-par02p.notify.windows.com":    true,
		"FCM.googleapis.com":                true,
		"169.254.169.254":                   false,
		"localhost":                         false,
		"evil.example.com":                  false,
		"fcm.googleapis.com.evil.example":   false,
		"evilpush.apple.com":                false,
	} {
		if got := pushServiceHost(host); got != want {
			t.Errorf("%s: got %v, want %v", host, got, want)
		}
	}
	s := &Server{}
	for raw, want := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":      true,
		"https://fcm.googleapis.com:8443/fcm/send/abc": false,
		"https://user@fcm.googleapis.com/fcm/send/abc": false,
		"http://fcm.googleapis.com/fcm/send/abc":       false,
		"https://10.0.0.1/x":                           false,
	} {
		if got := s.validPushEndpoint(raw); got != want {
			t.Errorf("%s: got %v, want %v", raw, got, want)
		}
	}
}
