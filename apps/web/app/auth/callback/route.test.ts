import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { CookieMethodsServer } from "@supabase/ssr";
import { GET } from "./route";

const { exchangeCodeForSession, sessionOptions } = vi.hoisted(() => ({
  exchangeCodeForSession: vi.fn(),
  // setAll に渡す Cookie 属性。Supabase の既定どおり secure は含めない。
  sessionOptions: { current: {} as Record<string, unknown> },
}));

vi.mock("@supabase/ssr", () => ({
  createServerClient: (_url: string, _key: string, options: { cookies: CookieMethodsServer }) => ({
    auth: {
      exchangeCodeForSession: async (code: string) => {
        await options.cookies.setAll?.([
          { name: "session", value: "token", options: sessionOptions.current },
        ], {});
        return exchangeCodeForSession(code);
      },
    },
  }),
}));

describe("auth callback redirect", () => {
  beforeEach(() => {
    sessionOptions.current = { path: "/", secure: true, sameSite: "none" };
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://auth.example");
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "test-key");
    exchangeCodeForSession.mockResolvedValue({ error: null });
  });

  afterEach(() => vi.unstubAllEnvs());

  async function callback(path?: string) {
    const request = new NextRequest("https://tracktalk.example/auth/callback?code=test-code");
    if (path !== undefined) request.cookies.set({ name: "tracktalk-next", value: path });
    return GET(request);
  }

  it("preserves a same-site path, query and fragment", async () => {
    const response = await callback(encodeURIComponent("/competitions?prefecture=13#results"));
    expect(response.headers.get("location")).toBe("/competitions?prefecture=13#results");
    expect(response.cookies.get("tracktalk-next")?.maxAge).toBe(0);
    expect(response.cookies.get("tracktalk-next")?.sameSite).toBe("lax");
    expect(response.cookies.get("session")).toMatchObject({ value: "token", sameSite: "lax", secure: true, path: "/" });
  });

  it.each([
    undefined,
    "https://evil.example/phishing",
    "//evil.example/phishing",
    "/\\evil.example/phishing",
    "javascript:alert(1)",
    "%2F%2Fevil.example",
    "%2F%09%2Fevil.example",
    "%2F%0A%2Fevil.example",
    "%2F%0D%2Fevil.example",
    "/login",
  ])("falls back to the site root for %s", async (path) => {
    const response = await callback(path);
    expect(response.headers.get("location")).toBe("/");
  });

  it("falls back to root when normalization leaves two leading slashes in the path", async () => {
    const response = await callback("/a/..//evil.example/phishing");
    const destination = new URL(response.headers.get("location")!, "https://tracktalk.example");
    expect(destination.origin).toBe("https://tracktalk.example");
    expect(destination.pathname).toBe("/");
  });

  it("returns to login if the code exchange fails", async () => {
    exchangeCodeForSession.mockResolvedValue({ error: new Error("invalid code") });
    const response = await callback("/me");
    const destination = new URL(response.headers.get("location")!, "https://tracktalk.example");
    expect(destination.origin).toBe("https://tracktalk.example");
    expect(destination.pathname).toBe("/login");
    expect(response.cookies.get("tracktalk-next")?.maxAge).toBe(0);
    expect(response.cookies.get("tracktalk-next")?.sameSite).toBe("lax");
  });

  it.each(["success", "missing code", "exchange failure", "missing config"])(
    "keeps the browser host when Netlify uses an internal request URL: %s",
    async (scenario) => {
      if (scenario === "exchange failure") {
        exchangeCodeForSession.mockResolvedValue({ error: new Error("invalid code") });
      }
      if (scenario === "missing config") vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "");
      const query = scenario === "missing code" ? "" : "?code=test-code";
      const request = new NextRequest(`https://main--track-talk.netlify.app/auth/callback${query}`, {
        headers: { "x-forwarded-host": "evil.example", host: "evil.example" },
      });
      request.cookies.set({ name: "tracktalk-next", value: encodeURIComponent("/me?tab=posts#recent") });
      const response = await GET(request);
      const location = response.headers.get("location")!;
      expect(response.status).toBe(307);
      expect(location.startsWith("/")).toBe(true);
      expect(location.startsWith("//")).toBe(false);
      // 同じレスポンスをどちらの公開ホストで受けても、そのホストから移動しない。
      for (const origin of ["https://track-talk.netlify.app", "http://localhost:3000"]) {
        const destination = new URL(location, `${origin}/auth/callback`);
        expect(destination.origin).toBe(origin);
        expect(destination.pathname).toBe(scenario === "success" ? "/me" : "/login");
      }
      if (scenario === "success") expect(location).toBe("/me?tab=posts#recent");
      expect(response.cookies.get("tracktalk-next")?.maxAge).toBe(0);
    },
  );

  describe("セッション Cookie の Secure 属性", () => {
    beforeEach(() => {
      sessionOptions.current = { path: "/", maxAge: 34560000 };
    });

    it("https では secure: true を付け、他の属性を保つ", async () => {
      const response = await GET(new NextRequest("https://tracktalk.example/auth/callback?code=test-code"));
      expect(response.cookies.get("session")).toMatchObject({
        value: "token", path: "/", maxAge: 34560000, sameSite: "lax", secure: true,
      });
      expect(response.headers.get("set-cookie")).toMatch(/session=token.*;\s*Secure/i);
    });

    it("http では secure を付けない", async () => {
      const response = await GET(new NextRequest("http://localhost:3000/auth/callback?code=test-code"));
      expect(response.cookies.get("session")).toMatchObject({
        value: "token", path: "/", maxAge: 34560000, sameSite: "lax",
      });
      expect(response.cookies.get("session")?.secure).toBeFalsy();
    });

    it("HttpOnly を付けない", async () => {
      for (const origin of ["https://tracktalk.example", "http://localhost:3000"]) {
        const response = await GET(new NextRequest(`${origin}/auth/callback?code=test-code`));
        expect(response.cookies.get("session")?.httpOnly).toBeFalsy();
        expect(response.headers.get("set-cookie")).not.toMatch(/httponly/i);
      }
    });
  });
});
