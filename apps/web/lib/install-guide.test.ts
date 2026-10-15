import { describe, expect, it } from "vitest";
import { detectInstallPlatform, installGuideDismissMs, isDismissed } from "./install-guide";

const iphoneSafari =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1";
const iphoneChrome =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/130.0 Mobile/15E148 Safari/604.1";
const iphoneLine =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Line/14.0.0";
const android =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36";
const mac =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15";

describe("detectInstallPlatform", () => {
  it("iPhone の Safari を判定する", () => {
    expect(detectInstallPlatform(iphoneSafari)).toBe("ios-safari");
  });
  it("iPhone の Chrome やアプリ内ブラウザは Safari 以外として扱う", () => {
    expect(detectInstallPlatform(iphoneChrome)).toBe("ios-other");
    expect(detectInstallPlatform(iphoneLine)).toBe("ios-other");
  });
  it("iPadOS(Macintosh UA + タッチ)を iOS として扱う", () => {
    expect(detectInstallPlatform(mac, 5)).toBe("ios-safari");
  });
  it("Android を判定する", () => {
    expect(detectInstallPlatform(android)).toBe("android");
  });
  it("PC は other", () => {
    expect(detectInstallPlatform(mac, 0)).toBe("other");
  });
});

describe("isDismissed", () => {
  const now = 1_000_000_000_000;
  it("未保存なら閉じていない扱い", () => {
    expect(isDismissed(null, now)).toBe(false);
    expect(isDismissed(Number.NaN, now)).toBe(false);
  });
  it("7日以内なら閉じている扱い", () => {
    expect(isDismissed(now - installGuideDismissMs + 1, now)).toBe(true);
  });
  it("7日を過ぎたら再表示する", () => {
    expect(isDismissed(now - installGuideDismissMs, now)).toBe(false);
  });
});
