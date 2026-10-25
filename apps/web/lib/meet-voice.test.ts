// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { connectMeetVoice } from "./meet-voice";

const sdk = vi.hoisted(() => ({
  connect: vi.fn(), disconnect: vi.fn(), microphone: vi.fn(),
  options: undefined as unknown,
  listeners: new Map<string, Set<(...args: unknown[]) => void>>(),
}));
vi.mock("livekit-client", () => ({
  Room: class {
    constructor(options: unknown) { sdk.options = options; }
    connect = sdk.connect;
    disconnect = sdk.disconnect;
    localParticipant = { setMicrophoneEnabled: sdk.microphone };
    on(event: string, callback: (...args: unknown[]) => void) {
      if (!sdk.listeners.has(event)) sdk.listeners.set(event, new Set());
      sdk.listeners.get(event)!.add(callback);
    }
    off(event: string, callback: (...args: unknown[]) => void) {
      sdk.listeners.get(event)?.delete(callback);
    }
  },
  RoomEvent: { TrackSubscribed: "subscribed", TrackUnsubscribed: "unsubscribed", Disconnected: "disconnected" },
  Track: { Kind: { Audio: "audio" } },
}));
function emit(event: string, value?: unknown) {
  for (const cb of sdk.listeners.get(event) ?? []) cb(value);
}
const credentials = { url: "wss://test.livekit.cloud", token: "private-token" };

beforeEach(() => {
  vi.clearAllMocks();
  sdk.listeners.clear();
  sdk.connect.mockResolvedValue(undefined);
  sdk.disconnect.mockResolvedValue(undefined);
});

describe("voice connection", () => {
  it("joins without requesting microphone access and removes remote audio on exit", async () => {
    const host = document.createElement("div");
    const connection = await connectMeetVoice(credentials, host, new AbortController().signal);
    expect(sdk.connect).toHaveBeenCalledWith(credentials.url, credentials.token, { autoSubscribe: true });
    expect(sdk.microphone).not.toHaveBeenCalled();
    expect(sdk.options).toMatchObject({ publishDefaults: { stopMicTrackOnMute: true } });
    const element = document.createElement("audio");
    const track = { kind: "audio", attach: vi.fn(() => element), detach: vi.fn() };
    emit("subscribed", track);
    expect(host.children.length).toBe(1);
    await connection.disconnect();
    expect(track.detach).toHaveBeenCalledWith(element);
    expect(host.children.length).toBe(0);
    expect(sdk.disconnect).toHaveBeenCalledWith(true);
  });

  it("stops the room when navigation or logout aborts an in-progress join", async () => {
    let resolve!: () => void;
    sdk.connect.mockImplementation(() => new Promise<void>((done) => { resolve = done; }));
    const controller = new AbortController();
    const pending = connectMeetVoice(credentials, document.createElement("div"), controller.signal);
    await vi.waitFor(() => expect(sdk.connect).toHaveBeenCalled());
    controller.abort();
    resolve();
    await expect(pending).rejects.toThrow();
    expect(sdk.disconnect).toHaveBeenCalledWith(true);
    expect(sdk.microphone).not.toHaveBeenCalled();
  });

  it("cleans up a failed connection", async () => {
    sdk.connect.mockRejectedValue(new Error("connection failed"));
    await expect(connectMeetVoice(credentials, document.createElement("div"), new AbortController().signal)).rejects.toThrow("connection failed");
    expect(sdk.disconnect).toHaveBeenCalledWith(true);
    expect([...sdk.listeners.values()].every((callbacks) => callbacks.size === 0)).toBe(true);
  });

  it("never connects after cancellation", async () => {
    const controller = new AbortController();
    controller.abort();
    await expect(connectMeetVoice(credentials, document.createElement("div"), controller.signal)).rejects.toThrow();
    expect(sdk.connect).not.toHaveBeenCalled();
  });
});
