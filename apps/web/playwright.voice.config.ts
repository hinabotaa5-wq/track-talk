import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e", testMatch: "meet-voice.spec.ts", workers: 1,
  use: { baseURL: "http://127.0.0.1:3127", viewport: { width: 320, height: 820 } },
  webServer: {
    command: "npm run dev -- --hostname 127.0.0.1 --port 3127",
    url: "http://127.0.0.1:3127", timeout: 120000, reuseExistingServer: false,
    env: { TRACKTALK_VOICE_E2E: "1", NEXT_PUBLIC_MEET_VOICE_ENABLED: "1", NEXT_PUBLIC_SUPABASE_URL: "http://127.0.0.1:9999", NEXT_PUBLIC_SUPABASE_ANON_KEY: "voice-e2e" },
  },
});
