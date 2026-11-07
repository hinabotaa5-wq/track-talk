/** @vitest-environment jsdom */
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }));

import { apiFetch } from "@/lib/api";
import { refreshPushSubscription } from "./push";

const mockedFetch = vi.mocked(apiFetch);

function stubPlatform() {
  vi.stubGlobal("PushManager", function PushManager() {});
  vi.stubGlobal("Notification", { permission: "granted" });
}

function stubServiceWorker(subscription: { endpoint: string; toJSON: () => unknown } | null, publicKey = "AQID") {
  const created = subscription ?? { endpoint: "https://example.com/new", toJSON: () => ({ endpoint: "https://example.com/new" }) };
  const subscribe = vi.fn(async () => created);
  const manager = {
    getSubscription: vi.fn(async () => subscription),
    subscribe,
  };
  const registration = { pushManager: manager };
  const register = vi.fn(async () => registration);
  Object.defineProperty(window.navigator, "serviceWorker", {
    value: { register, ready: Promise.resolve(registration), getRegistration: async () => registration },
    configurable: true,
  });
  void publicKey;
  return { register, manager, subscribe };
}

function fetchFor(path: string) {
  return mockedFetch.mock.calls.map((call) => call[0]).filter((p) => p === path).length;
}

beforeEach(() => {
  vi.unstubAllGlobals();
  mockedFetch.mockReset();
  localStorage.clear();
  stubPlatform();
  mockedFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/me/push-subscriptions/status") return { ever_subscribed: true };
    if (path === "/v1/push/public-key") return { public_key: "AQID" };
    return {};
  });
});

describe("refreshPushSubscription", () => {
  it("resubscribes on login when permission is granted but the subscription is gone", async () => {
    const { subscribe } = stubServiceWorker(null);
    await refreshPushSubscription("token");
    expect(subscribe).toHaveBeenCalledTimes(1);
    expect(fetchFor("/v1/me/push-subscriptions/status")).toBe(1);
    expect(fetchFor("/v1/push/public-key")).toBe(1);
    expect(fetchFor("/v1/me/push-subscriptions")).toBe(1);
  });

  it("does not resubscribe an account that never enabled notifications", async () => {
    mockedFetch.mockImplementation(async (path: string) => {
      if (path === "/v1/me/push-subscriptions/status") return { ever_subscribed: false };
      if (path === "/v1/push/public-key") return { public_key: "AQID" };
      return {};
    });
    const { subscribe } = stubServiceWorker(null);
    await refreshPushSubscription("token");
    expect(subscribe).not.toHaveBeenCalled();
    expect(fetchFor("/v1/me/push-subscriptions")).toBe(0);
  });

  it("does not resubscribe when permission is undecided or denied", async () => {
    for (const permission of ["default", "denied"] as const) {
      vi.stubGlobal("Notification", { permission });
      const { subscribe, register } = stubServiceWorker(null);
      await refreshPushSubscription("token");
      expect(subscribe).not.toHaveBeenCalled();
      expect(register).not.toHaveBeenCalled();
      expect(mockedFetch).not.toHaveBeenCalled();
      mockedFetch.mockClear();
    }
  });

  it("still re-registers an existing subscription without asking the status endpoint", async () => {
    const existing = {
      endpoint: "https://example.com/sub",
      toJSON: () => ({ endpoint: "https://example.com/sub" }),
      options: { userVisibleOnly: true, applicationServerKey: Uint8Array.from([1, 2, 3]).buffer },
    };
    stubServiceWorker(existing);
    await refreshPushSubscription("token");
    expect(fetchFor("/v1/me/push-subscriptions/status")).toBe(0);
    expect(fetchFor("/v1/me/push-subscriptions")).toBe(1);
  });
});
