import { describe, expect, it } from "vitest";
import { umamiBeforeSendName, umamiBeforeSendScript } from "./analytics";

type BeforeSend = (type: string, payload: { url?: string; referrer?: string }) => { url?: string; referrer?: string };

function load(): BeforeSend {
  const win: Record<string, unknown> = {};
  new Function("window", umamiBeforeSendScript)(win);
  return win[umamiBeforeSendName] as BeforeSend;
}

describe("umamiBeforeSend", () => {
  const token = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG";

  it("hides the guardian token in the url and referrer", () => {
    const send = load();
    expect(send("event", { url: `/guardian/${token}`, referrer: `https://tracktalk.jp/guardian/${token}/personal/1` })).toEqual({
      url: "/guardian/:token",
      referrer: "https://tracktalk.jp/guardian/:token/personal/1",
    });
    expect(send("event", { url: `/guardian/${token}?x=1` }).url).toBe("/guardian/:token?x=1");
  });

  it("keeps other pages as they are", () => {
    const send = load();
    expect(send("event", { url: "/guardian/paid" }).url).toBe("/guardian/paid");
    expect(send("event", { url: "/posts/123" }).url).toBe("/posts/123");
  });
});
