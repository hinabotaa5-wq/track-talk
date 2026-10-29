import { afterEach, describe, expect, it, vi } from "vitest";
import { trackEvent, trackSuccessfulMutation, umamiBeforeSendName, umamiBeforeSendScript } from "./analytics";

afterEach(() => vi.unstubAllGlobals());

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
    expect(send("event", { url: `/guardian/${token}`, referrer: `https://track-talk.netlify.app/guardian/${token}/personal/1` })).toEqual({
      url: "/guardian/:token",
      referrer: "https://track-talk.netlify.app/guardian/:token/personal/1",
    });
    expect(send("event", { url: `/guardian/${token}?x=1` }).url).toBe("/guardian/:token");
  });

  it("keeps other pages as they are", () => {
    const send = load();
    expect(send("event", { url: "/guardian/paid" }).url).toBe("/guardian/paid");
    expect(send("event", { url: "/posts/123" }).url).toBe("/posts/123");
  });

  it("removes search terms and authentication query/hash values", () => {
    expect(load()("event", {
      url: "/?q=private+search#secret",
      referrer: "https://track-talk.netlify.app/auth/callback?code=secret#access_token=secret",
    })).toEqual({ url: "/", referrer: "https://track-talk.netlify.app/auth/callback" });
  });
});

describe("operation analytics", () => {
  function tracker(hostname = "track-talk.netlify.app") {
    const track = vi.fn();
    vi.stubGlobal("window", { location: { hostname }, umami: { track } });
    return track;
  }

  it.each([
    ["/v1/posts", "POST", "post_created"],
    ["/v1/injury/groups/knee/posts", "POST", "post_created"],
    ["/v1/posts/123/likes", "POST", "post_liked"],
    ["/v1/me/race-records", "POST", "race_record_created"],
    ["/v1/me/race-records/123", "PATCH", "race_record_updated"],
  ])("counts %s %s without sending payloads or identifiers", (path, method, event) => {
    const track = tracker();
    trackSuccessfulMutation(path, method);
    expect(track).toHaveBeenCalledExactlyOnceWith(event);
  });

  it.each([
    ["/v1/posts", "GET"], ["/v1/posts/123", "PATCH"],
    ["/v1/posts/123/likes", "DELETE"], ["/v1/me/race-records/123", "DELETE"],
    ["/v1/guardian/secret/messages", "POST"], ["/v1/uploads/image", "POST"],
  ])("does not count unrelated calls or reversals: %s %s", (path, method) => {
    const track = tracker();
    trackSuccessfulMutation(path, method);
    expect(track).not.toHaveBeenCalled();
  });

  it("does not send local development or preview events", () => {
    const track = tracker("localhost");
    trackEvent("search_submitted");
    expect(track).not.toHaveBeenCalled();
  });

  it("tracks the verified Netlify production hostname", () => {
    const track = tracker("track-talk.netlify.app");
    trackEvent("search_submitted");
    expect(track).toHaveBeenCalledExactlyOnceWith("search_submitted");
  });

  it("works without a browser or a loaded tracker", () => {
    expect(() => trackEvent("search_submitted")).not.toThrow();
    vi.stubGlobal("window", { location: { hostname: "track-talk.netlify.app" } });
    expect(() => trackEvent("search_submitted")).not.toThrow();
  });

  it("does not propagate synchronous or asynchronous tracker failures", async () => {
    const track = tracker();
    track.mockImplementationOnce(() => { throw new Error("blocked"); });
    expect(() => trackEvent("search_submitted")).not.toThrow();
    track.mockRejectedValueOnce(new Error("offline"));
    trackEvent("search_submitted");
    await Promise.resolve();
  });
});
