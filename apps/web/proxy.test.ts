import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { CookieMethodsServer } from "@supabase/ssr";
import { proxy } from "./proxy";

const { getClaims, sessionOptions } = vi.hoisted(() => ({
  getClaims: vi.fn(),
  // setAll に渡す Cookie 属性。Supabase の既定どおり secure は含めない。
  sessionOptions: { current: {} as Record<string, unknown> },
}));

vi.mock("@supabase/ssr", () => ({
  createServerClient: (_url: string, _key: string, options: { cookies: CookieMethodsServer }) => ({
    auth: {
      getClaims: async () => {
        await options.cookies.setAll?.([
          { name: "session", value: "refreshed", options: sessionOptions.current },
        ], { "Cache-Control": "no-store" });
        return getClaims();
      },
    },
  }),
}));

describe("proxy session cookies", () => {
  beforeEach(() => {
    sessionOptions.current = { path: "/", secure: true, sameSite: "none" };
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

  describe.each([true, false])("Secure 属性（authenticated=%s）", (authenticated) => {
    beforeEach(() => {
      sessionOptions.current = { path: "/", maxAge: 34560000 };
      getClaims.mockResolvedValue({ data: { claims: authenticated ? { sub: "user" } : null } });
    });

    it("https のリクエストでは secure: true を付け、他の属性を保つ", async () => {
      const response = await proxy(new NextRequest("https://tracktalk.example/me"));
      expect(response.cookies.get("session")).toMatchObject({
        value: "refreshed", path: "/", maxAge: 34560000, sameSite: "lax", secure: true,
      });
      expect(response.headers.get("set-cookie")).toMatch(/;\s*Secure/i);
    });

    it("http のリクエストでは secure を付けない", async () => {
      const response = await proxy(new NextRequest("http://localhost:3000/me"));
      expect(response.cookies.get("session")).toMatchObject({
        value: "refreshed", path: "/", maxAge: 34560000, sameSite: "lax",
      });
      expect(response.cookies.get("session")?.secure).toBeFalsy();
      expect(response.headers.get("set-cookie")).not.toMatch(/;\s*Secure/i);
    });

    it("HttpOnly を付けない", async () => {
      for (const origin of ["https://tracktalk.example", "http://localhost:3000"]) {
        const response = await proxy(new NextRequest(`${origin}/me`));
        expect(response.cookies.get("session")?.httpOnly).toBeFalsy();
        expect(response.headers.get("set-cookie")).not.toMatch(/httponly/i);
      }
    });
  });
});
