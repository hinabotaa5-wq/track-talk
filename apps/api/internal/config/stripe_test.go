package config

import "testing"

func TestValidateStripe(t *testing.T) {
	ok := Config{
		StripeSecretKey:     "sk_test_x",
		StripeWebhookSecret: "whsec_x",
		WebBaseURL:          "http://localhost:3000",
	}
	live := ok
	live.StripeSecretKey = "rk_live_x"
	live.WebBaseURL = "https://example.jp"
	live.CoachingTickSecret = "tick"

	with := func(c Config, mutate func(*Config)) Config {
		mutate(&c)
		return c
	}
	cases := map[string]struct {
		cfg  Config
		want bool
		mode string
	}{
		"鍵が無ければ確かめない(デモ版)": {Config{}, true, ""},
		"テスト鍵":                     {ok, true, "test"},
		"公開可能キー":                   {with(ok, func(c *Config) { c.StripeSecretKey = "pk_test_x" }), false, "test"},
		"Webhook の鍵が無い":            {with(ok, func(c *Config) { c.StripeWebhookSecret = "" }), false, "test"},
		"Webhook の鍵の形が違う":          {with(ok, func(c *Config) { c.StripeWebhookSecret = "abc" }), false, "test"},
		"Connect の鍵の形が違う":          {with(ok, func(c *Config) { c.StripeConnectWebhookSecret = "abc" }), false, "test"},
		"Connect の鍵あり":             {with(ok, func(c *Config) { c.StripeConnectWebhookSecret = "whsec_y" }), true, "test"},
		"本番鍵":                      {live, true, "live"},
		"本番鍵で https でない":           {with(live, func(c *Config) { c.WebBaseURL = "http://example.jp" }), false, "live"},
		"本番鍵で tick の秘密が無い":         {with(live, func(c *Config) { c.CoachingTickSecret = "" }), false, "live"},
		"テスト鍵なら http と tick なしでよい": {with(ok, func(c *Config) { c.CoachingTickSecret = "" }), true, "test"},
	}
	for name, c := range cases {
		if got := c.cfg.ValidateStripe() == nil; got != c.want {
			t.Errorf("%s: got %v want %v (%v)", name, got, c.want, c.cfg.ValidateStripe())
		}
		if got := c.cfg.StripeMode(); got != c.mode && c.cfg.StripeSecretKey != "" {
			t.Errorf("%s: mode %q want %q", name, got, c.mode)
		}
	}
}
