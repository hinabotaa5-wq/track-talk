import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { CookieMethodsServer } from "@supabase/ssr";
import { proxy } from "./proxy";

const { getClaims } = vi.hoisted(() => ({ getClaims: vi.fn() }));

vi.mock("@supabase/ssr", () => ({
  createServerClient: (_url: string, _key: string, options: { cookies: CookieMethodsServer }) => ({
    auth: {
      getClaims: async () => {
        await options.cookies.setAll?.([
          { name: "session", value: "refreshed", options: { path: "/", secure: true, sameSite: "none" } },
        ], { "Cache-Control": "no-store" });
        return getClaims();
      },
    },
  }),
}));

describe("proxy session cookies", () => {
  beforeEach(() => {
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://auth.example");
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "test-key");
  });

  afterEach(() => vi.unstubAllEnvs());

  it.each([true, false])("sets SameSite=Lax after refresh when authenticated=%s", async (authenticated) => {
    getClaims.mockResolvedValue({ data: { claims: authenticated ? { sub: "user" } : null } });
    const request = new NextRequest("https://tracktalk.example/me");
    const response = await proxy(request);

    expect(request.cookies.get("session")?.value).toBe("refreshed");
    expect(response.cookies.get("session")).toMatchObject({
      value: "refreshed", path: "/", secure: true, sameSite: "lax",
    });
    expect(response.headers.get("set-cookie")).toContain("SameSite=lax");
    expect(response.headers.get("cache-control")).toBe("no-store");
    if (authenticated) {
      expect(response.headers.get("location")).toBeNull();
    } else {
      expect(response.headers.get("location")).toBe("https://tracktalk.example/login?next=%2Fme");
    }
  });
});
