import { expect, it } from "vitest";
import { detectVideoUrl } from "./video-url";
import { parseXPostUrl, xPostPermalink } from "./x-url";

it.each([
  "https://x.com/runner/status/123456789?s=46",
  "https://twitter.com/runner/status/123456789/photo/1",
  "https://mobile.twitter.com/runner/status/123456789",
  "https://x.com/i/web/status/123456789",
  "https://x.com/i/status/123456789",
])("Xの投稿URLを同じIDに正規化する: %s", (url) => {
  expect(parseXPostUrl(url)).toBe("123456789");
  expect(detectVideoUrl(url)).toBe("x");
});
it.each(["https://x.com/runner", "https://x.com.evil.test/runner/status/123", "https://x.com@evil.test/runner/status/123", "https://someone@x.com/runner/status/123", "https://x.com:444/runner/status/123", "https://x.com/runner/status/nope", "javascript:alert(1)"])("投稿以外や偽のURLを受け付けない: %s", (url) => expect(parseXPostUrl(url)).toBeNull());
it("リンクをXの投稿に固定する", () => {
  expect(xPostPermalink("123")).toBe("https://x.com/i/status/123");
  expect(xPostPermalink("//evil.test")).toBeNull();
});
