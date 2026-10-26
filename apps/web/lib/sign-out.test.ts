import { afterEach, describe, expect, it, vi } from "vitest";
import { clearPersonalStorage, unsubscribePushOnThisDevice } from "./sign-out";

function memoryStorage(entries: Record<string, string>) {
  const data = new Map(Object.entries(entries));
  return {
    get length() {
      return data.size;
    },
    key: (i: number) => [...data.keys()][i] ?? null,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
    clear: () => data.clear(),
    keys: () => [...data.keys()],
  };
}

describe("clearPersonalStorage", () => {
  it("removes per-user keys and keeps device settings", () => {
    const storage = memoryStorage({
      "injury-celebrated:sprain": "1",
      "injury-celebrated:jumpers-knee": "1",
      "review-attrs": '{"size":"26.5"}',
      "competition-prefecture": "13",
    });
    clearPersonalStorage(storage);
    expect(storage.keys()).toEqual(["competition-prefecture"]);
  });
});

describe("unsubscribePushOnThisDevice", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("tells the API and drops the browser subscription", async () => {
    const unsubscribe = vi.fn().mockResolvedValue(true);
    const subscription = { endpoint: "https://fcm.googleapis.com/fcm/send/abc", unsubscribe };
    vi.stubGlobal("navigator", {
      serviceWorker: { getRegistration: vi.fn().mockResolvedValue({ pushManager: { getSubscription: vi.fn().mockResolvedValue(subscription) } }) },
    });
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await unsubscribePushOnThisDevice("tok");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/v1/me/push-subscriptions");
    expect(init.method).toBe("DELETE");
    expect(JSON.parse(init.body)).toEqual({ endpoint: subscription.endpoint });
    expect(unsubscribe).toHaveBeenCalled();
  });

  it("still drops the browser subscription when the API call fails", async () => {
    const unsubscribe = vi.fn().mockResolvedValue(true);
    vi.stubGlobal("navigator", {
      serviceWorker: { getRegistration: vi.fn().mockResolvedValue({ pushManager: { getSubscription: vi.fn().mockResolvedValue({ endpoint: "e", unsubscribe }) } }) },
    });
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    await unsubscribePushOnThisDevice("tok");
    expect(unsubscribe).toHaveBeenCalled();
  });

  it("does nothing without a service worker", async () => {
    vi.stubGlobal("navigator", {});
    await expect(unsubscribePushOnThisDevice(null)).resolves.toBeUndefined();
  });
});

it("通知サービスが応答しなくてもログアウトの前処理は2秒で終わる", async () => {
  vi.useFakeTimers();
  vi.stubGlobal("navigator", { serviceWorker: { getRegistration: () => new Promise(() => {}) } });
  try {
    const cleanup = unsubscribePushOnThisDevice("tok");
    await vi.advanceTimersByTimeAsync(2000);
    await expect(cleanup).resolves.toBeUndefined();
  } finally { vi.useRealTimers(); vi.unstubAllGlobals(); }
});
