import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CloudinaryUploadError,
  cloudinaryErrorMessage,
  uploadVideoToCloudinary,
  videoProcessingTimeoutMessage,
  waitForVideoReady,
  type CloudinarySignature,
} from "./cloudinary-upload";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

const signed: CloudinarySignature = {
  cloud_name: "demo",
  api_key: "k",
  timestamp: 1700000000,
  public_id: "track-talk/v1",
  signature: "s",
  upload_preset: "track-talk-video-async",
  allowed_formats: "mov,mp4,webm",
  resource_type: "video",
  eager: "c_limit,w_1280,h_1280,q_auto/mp4",
  eager_async: "true",
};

// XMLHttpRequest の代わり。送られた FormData を取っておき、決めた応答を返す。
function stubXHR(status: number, response: unknown) {
  const sent: FormData[] = [];
  class FakeXHR {
    status = status;
    responseText = JSON.stringify(response);
    upload = { onprogress: null };
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    onabort: (() => void) | null = null;
    open() {}
    abort() {}
    send(body: FormData) {
      sent.push(body);
      queueMicrotask(() => this.onload?.());
    }
  }
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  return sent;
}

function stubStatus(readies: boolean[]) {
  const calls: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init: RequestInit) => {
      calls.push(JSON.parse(String(init.body)));
      const ready = readies[Math.min(calls.length - 1, readies.length - 1)];
      return new Response(JSON.stringify({ ready }), { status: 200 });
    }),
  );
  return calls;
}

describe("uploadVideoToCloudinary", () => {
  it("sends eager parameters and waits until the derived videos are ready", async () => {
    vi.useFakeTimers();
    const sent = stubXHR(200, { public_id: "track-talk/v1" });
    const calls = stubStatus([false, false, true]);
    const file = new File(["x"], "a.mov", { type: "video/quicktime" });
    const done = uploadVideoToCloudinary(file, signed, "tok", () => {});
    await vi.runAllTimersAsync();
    await expect(done).resolves.toEqual({ public_id: "track-talk/v1" });
    expect(sent[0].get("eager")).toBe(signed.eager);
    expect(sent[0].get("eager_async")).toBe("true");
    expect(calls).toEqual([{ public_id: "track-talk/v1" }, { public_id: "track-talk/v1" }, { public_id: "track-talk/v1" }]);
  });

  it("does not wait when the signature has no eager (older API)", async () => {
    const sent = stubXHR(200, { public_id: "track-talk/v1" });
    const calls = stubStatus([true]);
    const file = new File(["x"], "a.mov", { type: "video/quicktime" });
    await uploadVideoToCloudinary(file, { ...signed, eager: undefined, eager_async: undefined }, "tok", () => {});
    expect(sent[0].get("eager")).toBeNull();
    expect(calls).toHaveLength(0);
  });

  it("keeps Cloudinary's raw message while showing a friendly one", async () => {
    const raw = "Video is too large to process synchronously, please use an eager transformation with eager_async=true to resolve";
    stubXHR(400, { error: { message: raw } });
    vi.spyOn(console, "error").mockImplementation(() => {});
    const file = new File(["x"], "a.mov", { type: "video/quicktime" });
    const err = await uploadVideoToCloudinary(file, signed, "tok", () => {}).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(CloudinaryUploadError);
    expect((err as CloudinaryUploadError).message).toBe("ファイルサイズが大きすぎます");
    expect((err as CloudinaryUploadError).detail).toBe(raw);
    expect(console.error).toHaveBeenCalledWith("Cloudinary upload failed", 400, raw);
    expect(cloudinaryErrorMessage(raw)).toBe("ファイルサイズが大きすぎます");
  });
});

describe("waitForVideoReady", () => {
  it("gives up after about ten minutes", async () => {
    vi.useFakeTimers();
    stubStatus([false]);
    let clock = 0;
    const done = waitForVideoReady("track-talk/v1", "tok", undefined, () => clock).catch((e: unknown) => e);
    for (let i = 0; i < 200 && clock < 11 * 60 * 1000; i++) {
      clock += 10000;
      await vi.advanceTimersByTimeAsync(10000);
    }
    const err = await done;
    expect((err as Error).message).toBe(videoProcessingTimeoutMessage);
  });
});
