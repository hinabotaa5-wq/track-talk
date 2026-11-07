import { describe, expect, it, vi } from "vitest";
import { isPwnedPassword, matchesPwnedRangeBody } from "./pwned-password";

// "password" の SHA-1（大文字）：5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8（sha1sum・openssl で実測）
const passwordSuffix = "1E4C9B93F3F0682250B6CF8331B7EE68FD8";

describe("matchesPwnedRangeBody", () => {
  it("接尾辞の一致で漏洩と判定する", () => {
    expect(matchesPwnedRangeBody(`${passwordSuffix}:100\nABCDEF:2`, passwordSuffix)).toBe(true);
  });

  it("一致がなければ通す", () => {
    expect(matchesPwnedRangeBody("AAAAA:3\nBBBBB:7", passwordSuffix)).toBe(false);
    expect(matchesPwnedRangeBody("", passwordSuffix)).toBe(false);
  });
});

describe("isPwnedPassword", () => {
  it("既知の漏洩パスワード（password）でブロックする", async () => {
    const fetchFn = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(`${passwordSuffix}:9999999\nOTHER:1`),
    });
    await expect(
      isPwnedPassword("password", { fetchFn: fetchFn as unknown as typeof fetch, timeoutMs: 1000 }),
    ).resolves.toBe(true);
    const url = String(fetchFn.mock.calls[0][0]);
    // k-anonymity：送るのは先頭5文字だけ。
    expect(url).toBe("https://api.pwnedpasswords.com/range/5BAA6");
    expect(fetchFn.mock.calls[0][1].headers).toEqual({ "Add-Padding": "true" });
  });

  it("ランダムな強いパスワードは通過する", async () => {
    const fetchFn = vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve("AAAAA:2\nBBBBB:3") });
    await expect(
      isPwnedPassword("xQ9#vLm2$kR4tW8z!", {
        fetchFn: fetchFn as unknown as typeof fetch,
        hashFn: () => Promise.resolve("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
        timeoutMs: 1000,
      }),
    ).resolves.toBe(false);
  });

  it("HIBP障害では fail-open で登録できる", async () => {
    const failing = vi.fn().mockRejectedValue(new Error("network down"));
    await expect(
      isPwnedPassword("password", { fetchFn: failing as unknown as typeof fetch, timeoutMs: 1000 }),
    ).resolves.toBe(false);

    const badStatus = vi.fn().mockResolvedValue({ ok: false, text: () => Promise.resolve("") });
    await expect(
      isPwnedPassword("password", { fetchFn: badStatus as unknown as typeof fetch, timeoutMs: 1000 }),
    ).resolves.toBe(false);
  });
});
