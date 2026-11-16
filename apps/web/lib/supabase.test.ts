import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createBrowserClient } from "@supabase/ssr";
import { createClient } from "./supabase";

vi.mock("@supabase/ssr", () => ({ createBrowserClient: vi.fn(() => ({})) }));

function lastOptions() {
  const calls = vi.mocked(createBrowserClient).mock.calls;
  return calls[calls.length - 1][2] as { cookieOptions?: Record<string, unknown> };
}

describe("ブラウザ側 Supabase クライアントの Cookie 属性", () => {
  beforeEach(() => {
    vi.mocked(createBrowserClient).mockClear();
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://auth.example");
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "test-key");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("https 環境では cookieOptions.secure が true", () => {
    vi.stubGlobal("window", { location: { protocol: "https:" } });
    createClient();
    expect(lastOptions().cookieOptions?.secure).toBe(true);
  });

  it("http 環境では cookieOptions.secure が false", () => {
    vi.stubGlobal("window", { location: { protocol: "http:" } });
    createClient();
    expect(lastOptions().cookieOptions?.secure).toBe(false);
  });

  it("window がない（SSR）でも例外にならず false", () => {
    vi.stubGlobal("window", undefined);
    expect(() => createClient()).not.toThrow();
    expect(lastOptions().cookieOptions?.secure).toBe(false);
  });

  it("httpOnly を指定しない", () => {
    vi.stubGlobal("window", { location: { protocol: "https:" } });
    createClient();
    expect(lastOptions().cookieOptions).not.toHaveProperty("httpOnly");
  });
});
