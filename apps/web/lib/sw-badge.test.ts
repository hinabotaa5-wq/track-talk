import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it, vi } from "vitest";

type PushEvent = { data: { json: () => unknown }; waitUntil: (work: Promise<unknown>) => void };

function runWorker(options: {
  binding?: string;
  badge?: { setAppBadge?: (...args: never[]) => unknown; clearAppBadge?: (...args: never[]) => unknown };
  showNotification?: (...args: never[]) => unknown;
}) {
  let push!: (event: PushEvent) => void;
  const showNotification = vi.fn(options.showNotification ?? (async () => undefined));
  const context = {
    self: {
      navigator: {
        locks: { request: (_: string, work: () => Promise<void>) => work() },
        ...(options.badge ?? {}),
      },
      addEventListener: (event: string, handler: (event: PushEvent) => void) => {
        if (event === "push") push = handler;
      },
      registration: { showNotification },
    },
    caches: {
      open: async () => ({
        match: async () => (options.binding ? { json: async () => ({ subscriptionId: options.binding }) } : undefined),
      }),
    },
  };
  runInNewContext(readFileSync(new URL("../public/sw.js", import.meta.url), "utf8"), context);
  return { push, showNotification };
}

async function deliver(push: (event: PushEvent) => void, payload: unknown): Promise<{ ok: boolean }> {
  let work: Promise<unknown> | undefined;
  push({ data: { json: () => payload }, waitUntil: (pending) => { work = pending; } });
  try {
    await work;
    return { ok: true };
  } catch {
    return { ok: false };
  }
}

describe("service worker app badge", () => {
  it("applies the unread count from the push payload", async () => {
    const setAppBadge = vi.fn(async () => undefined);
    const { push, showNotification } = runWorker({ binding: "sub-1", badge: { setAppBadge } });
    const result = await deliver(push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1", unread_count: 3 });
    expect(result.ok).toBe(true);
    expect(showNotification).toHaveBeenCalledTimes(1);
    expect(setAppBadge).toHaveBeenCalledWith(3);
  });

  it("clears the badge when the unread count is zero", async () => {
    const setAppBadge = vi.fn(async () => undefined);
    const clearAppBadge = vi.fn(async () => undefined);
    const { push, showNotification } = runWorker({ binding: "sub-1", badge: { setAppBadge, clearAppBadge } });
    await deliver(push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1", unread_count: 0 });
    expect(showNotification).toHaveBeenCalledTimes(1);
    expect(setAppBadge).not.toHaveBeenCalled();
    expect(clearAppBadge).toHaveBeenCalledTimes(1);
  });

  it("leaves the badge alone for missing or invalid counts and mismatched bindings", async () => {
    const setAppBadge = vi.fn(async () => undefined);
    const clearAppBadge = vi.fn(async () => undefined);
    const worker = runWorker({ binding: "sub-1", badge: { setAppBadge, clearAppBadge } });
    await deliver(worker.push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1" });
    await deliver(worker.push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1", unread_count: -1 });
    await deliver(worker.push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1", unread_count: "3" });
    await deliver(worker.push, { title: "TrackTalk", body: "通知", subscription_id: "other", unread_count: 3 });
    expect(setAppBadge).not.toHaveBeenCalled();
    expect(clearAppBadge).not.toHaveBeenCalled();
    expect(worker.showNotification).toHaveBeenCalledTimes(3);
  });

  it("still shows the notification when applying the badge fails", async () => {
    const { push, showNotification } = runWorker({
      binding: "sub-1",
      badge: { setAppBadge: async () => { throw new Error("denied"); } },
    });
    const result = await deliver(push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1", unread_count: 2 });
    expect(result.ok).toBe(true);
    expect(showNotification).toHaveBeenCalledTimes(1);
  });

  it("still applies the badge when showing the notification fails", async () => {
    const setAppBadge = vi.fn(async () => undefined);
    const { push, showNotification } = runWorker({
      binding: "sub-1",
      badge: { setAppBadge },
      showNotification: async () => { throw new Error("closed"); },
    });
    const result = await deliver(push, { title: "TrackTalk", body: "通知", subscription_id: "sub-1", unread_count: 2 });
    expect(result.ok).toBe(false);
    expect(showNotification).toHaveBeenCalledTimes(1);
    expect(setAppBadge).toHaveBeenCalledWith(2);
  });
});
