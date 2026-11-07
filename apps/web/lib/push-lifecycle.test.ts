// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { refreshPushSubscription } from "./push";
import { signOutOnThisDevice } from "./sign-out";

const mocks = vi.hoisted(() => ({ apiFetch: vi.fn(), getSession: vi.fn() }));
vi.mock("./api", () => ({ apiFetch: mocks.apiFetch }));
vi.mock("./supabase", () => ({ createClient: () => ({ auth: { getSession: mocks.getSession } }) }));

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it("binds an existing browser subscription only after registration succeeds", async () => {
  let account: string | null = "first";
  let stored: Response | undefined;
  mocks.getSession.mockImplementation(async () => ({
    data: { session: account ? { user: { id: account }, access_token: account } : null },
  }));
  const registration = {
    active: { state: "activated" },
    getNotifications: vi.fn().mockResolvedValue([]),
    pushManager: {
      getSubscription: async () => ({
        endpoint: "https://example.test/push",
        options: { applicationServerKey: new Uint8Array([0]).buffer },
        toJSON: () => ({ endpoint: "https://example.test/push" }),
      }),
    },
  };
  vi.stubGlobal("navigator", {
    locks: { request: (_: string, work: () => Promise<unknown>) => work() },
    serviceWorker: { getRegistration: async () => registration, register: async () => registration },
  });
  vi.stubGlobal("PushManager", {});
  vi.stubGlobal("Notification", { permission: "granted" });
  vi.stubGlobal("caches", {
    open: async () => ({
      match: async () => stored?.clone(),
      put: async (_: string, entry: Response) => { stored = entry; },
      delete: async () => { stored = undefined; },
    }),
  });
  let serial = 0;
  let fail = false;
  mocks.apiFetch.mockImplementation(async (path, options) => {
    if (path === "/v1/push/public-key") return { public_key: "AA" };
    if (options.method === "DELETE") {
      expect(stored).toBeUndefined();
      return undefined;
    }
    if (fail) throw new Error("registration failed");
    return { subscription_id: String(++serial) };
  });

  await expect(refreshPushSubscription("first")).resolves.toBe(true);
  expect(await stored?.clone().json()).toEqual({ userId: "first", subscriptionId: "1" });

  await signOutOnThisDevice("first", async () => {
    expect(stored).toBeUndefined();
    expect(mocks.apiFetch).toHaveBeenLastCalledWith("/v1/me/push-subscriptions", expect.objectContaining({
      method: "DELETE",
      token: "first",
    }));
    account = null;
  });
  await expect(refreshPushSubscription("first")).rejects.toThrow();
  account = "second";
  fail = true;
  await expect(refreshPushSubscription("second")).rejects.toThrow("registration failed");
  expect(stored).toBeUndefined();

  fail = false;
  await expect(refreshPushSubscription("second")).resolves.toBe(true);
  expect(await stored?.clone().json()).toEqual({ userId: "second", subscriptionId: "2" });
  await signOutOnThisDevice("second", async () => { account = null; });
  account = "first";
  await expect(refreshPushSubscription("first")).resolves.toBe(true);
  expect(await stored?.clone().json()).toEqual({ userId: "first", subscriptionId: "3" });
});
