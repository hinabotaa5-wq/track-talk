import { describe, expect, it } from "vitest";
import { contentSecurityPolicy } from "./next.config";

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
