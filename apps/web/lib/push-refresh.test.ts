/** @vitest-environment jsdom */
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ apiFetch: vi.fn(), getSession: vi.fn() }));
vi.mock("@/lib/api", () => ({ apiFetch: mocks.apiFetch }));
vi.mock("@/lib/supabase", () => ({ createClient: () => ({ auth: { getSession: mocks.getSession } }) }));

import { apiFetch } from "@/lib/api";
import { refreshPushSubscription } from "./push";

const mockedFetch = vi.mocked(apiFetch);
const mockedSession = mocks.getSession;

function stubPlatform(permission = "granted") {
  vi.stubGlobal("PushManager", function PushManager() {});
  vi.stubGlobal("Notification", { permission });
}

function stubServiceWorker(subscription: { endpoint: string; toJSON: () => unknown; options?: unknown } | null) {
  const created = subscription ?? { endpoint: "https://example.com/new", toJSON: () => ({ endpoint: "https://example.com/new" }) };
  const subscribe = vi.fn(async () => created);
  const manager = {
    getSubscription: vi.fn(async () => subscription),
    subscribe,
  };
  const registration = {
    pushManager: manager,
    getNotifications: vi.fn(async () => []),
    installing: null,
    waiting: null,
    active: { state: "activated", addEventListener: vi.fn(), removeEventListener: vi.fn() },
  };
  const register = vi.fn(async () => registration);
  vi.stubGlobal("navigator", {
    locks: { request: (_: string, work: () => Promise<unknown>) => work() },
    serviceWorker: { register, ready: Promise.resolve(registration), getRegistration: async () => registration },
  });
  return { register, manager, subscribe };
}

function stubCaches() {
  const store = new Map<string, Response>();
  vi.stubGlobal("caches", {
    open: async () => ({
      match: async (key: string) => store.get(key)?.clone(),
      put: async (key: string, entry: Response) => { store.set(key, entry); },
      delete: async (key: string) => { store.delete(key); },
    }),
  });
  return store;
}

function fetchCount(path: string) {
  return mockedFetch.mock.calls.filter((call) => String(call[0]).split("?")[0] === path).length;
}

beforeEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
  stubPlatform();
  stubServiceWorker(null);
  stubCaches();
  mockedSession.mockResolvedValue({
    data: { session: { user: { id: "user-1" }, access_token: "token" } },
    error: null,
  });
  mockedFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (path === "/v1/me/push-subscriptions/status") return { ever_subscribed: true };
    if (path === "/v1/push/public-key") return { public_key: "AQID" };
    if (options?.method === "POST") return { subscription_id: "sub-1" };
    return {};
  });
});

describe("refreshPushSubscription", () => {
  it("resubscribes on login when permission is granted but the subscription is gone", async () => {
    const { subscribe } = stubServiceWorker(null);
    await expect(refreshPushSubscription("user-1")).resolves.toBe(true);
    expect(subscribe).toHaveBeenCalledTimes(1);
    expect(fetchCount("/v1/me/push-subscriptions/status")).toBe(1);
    expect(fetchCount("/v1/push/public-key")).toBe(1);
    expect(fetchCount("/v1/me/push-subscriptions")).toBe(1);
  });

  it("does not resubscribe an account that never enabled notifications", async () => {
    mockedFetch.mockImplementation(async (path: string) => {
      if (path === "/v1/me/push-subscriptions/status") return { ever_subscribed: false };
      if (path === "/v1/push/public-key") return { public_key: "AQID" };
      return {};
    });
    const { subscribe } = stubServiceWorker(null);
    await expect(refreshPushSubscription("user-1")).resolves.toBe(false);
    expect(subscribe).not.toHaveBeenCalled();
    expect(fetchCount("/v1/me/push-subscriptions")).toBe(0);
  });

  it("does not resubscribe when permission is undecided or denied", async () => {
    for (const permission of ["default", "denied"] as const) {
      stubPlatform(permission);
      const { subscribe, register } = stubServiceWorker(null);
      stubCaches();
      await expect(refreshPushSubscription("user-1")).resolves.toBe(false);
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
    await expect(refreshPushSubscription("user-1")).resolves.toBe(true);
    expect(fetchCount("/v1/me/push-subscriptions/status")).toBe(0);
    expect(fetchCount("/v1/me/push-subscriptions")).toBe(1);
  });
});
