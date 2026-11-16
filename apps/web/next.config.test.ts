import { describe, expect, it } from "vitest";
import nextConfig, { contentSecurityPolicy } from "./next.config";

function connectSrc(policy: string): string {
  const directive = policy
    .split(";")
    .map((part) => part.trim())
    .find((part) => part.startsWith("connect-src "));
  if (!directive) throw new Error("connect-src がありません");
  return directive;
}

describe("contentSecurityPolicy", () => {
  it("connect-src に HIBP の range API 送信先を許可する（本体 #436）", () => {
    expect(connectSrc(contentSecurityPolicy())).toContain("https://api.pwnedpasswords.com");
  });

  it("既存の送信先を保つ", () => {
    const connect = connectSrc(contentSecurityPolicy());
    expect(connect).toContain("'self'");
    expect(connect).toContain("https://*.supabase.co");
    expect(connect).toContain("https://challenges.cloudflare.com");
  });
});

describe("redirects", () => {
  it("/coaches は /coaching へ（ちょうどだけ・暫定）、既存3件は不変", async () => {
    const redirects = await nextConfig.redirects!();
    expect(redirects).toEqual([
      { source: "/about", destination: "/guidelines", permanent: false },
      { source: "/posts", destination: "/", permanent: false },
      { source: "/coach/availability", destination: "/coach", permanent: false },
      { source: "/coaches", destination: "/coaching", permanent: false },
    ]);
  });

  it("詳細 /coaches/[id] と無料相談を巻き込むワイルドカードを使わない", async () => {
    const redirects = await nextConfig.redirects!();
    for (const { source } of redirects) expect(source).not.toMatch(/:|\*/);
  });
});
