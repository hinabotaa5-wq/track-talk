/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MeetVoicePanel } from "./meet-voice-panel";
const s = vi.hoisted(() => ({ token: "auth" as string | null, uid: "user" as string | null, full: false, api: vi.fn(), connect: vi.fn(), mic: vi.fn(), disconnect: vi.fn(), events: new Map<string, Set<() => void>>() }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token: s.token, session: s.uid ? { user: { id: s.uid } } : null }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => s.api(...args) }));
vi.mock("@/lib/meet-voice", () => ({ connectMeetVoice: (...args: unknown[]) => s.connect(...args) }));
vi.mock("livekit-client", () => ({ RoomEvent: { ParticipantConnected: "pc", ParticipantDisconnected: "pd", TrackMuted: "tm", TrackUnmuted: "tu", ActiveSpeakersChanged: "as", LocalTrackPublished: "lp", LocalTrackUnpublished: "lu", TrackSubscribed: "ts", AudioPlaybackStatusChanged: "audio", Reconnecting: "reconnecting", SignalReconnecting: "signal", Reconnected: "reconnected", Disconnected: "disconnected" } }));
let root: Root; let container: HTMLDivElement;
const click = async (text: string) => { const button = [...container.querySelectorAll("button")].find((b) => b.textContent === text); expect(button, text).toBeTruthy(); await act(async () => button!.click()); };
const settle = async () => { await act(async () => { await new Promise((done) => setTimeout(done, 10)); }); };
beforeEach(() => {
 (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
 vi.clearAllMocks(); s.events.clear(); s.token = "auth"; s.uid = "user"; s.full = false;
 const local = { identity: "connection", name: "自分", isSpeaking: false, isMicrophoneEnabled: false, setMicrophoneEnabled: s.mic };
 s.mic.mockImplementation(async (enabled: boolean) => { local.isMicrophoneEnabled = enabled; }); s.disconnect.mockResolvedValue(undefined);
 const room = { localParticipant: local, remoteParticipants: new Map([["other", { identity: "other", name: "参加者", isSpeaking: true, isMicrophoneEnabled: true, setVolume: vi.fn() }]]), canPlaybackAudio: true,
 on(event: string, fn: () => void) { if (!s.events.has(event)) s.events.set(event, new Set()); s.events.get(event)!.add(fn); }, off(event: string, fn: () => void) { s.events.get(event)?.delete(fn); }, startAudio: vi.fn().mockResolvedValue(undefined) };
 s.connect.mockResolvedValue({ room, disconnect: s.disconnect });
 s.api.mockImplementation(async (path: string) => path.endsWith("/voice") ? { enabled: true, full: s.full, participants: 1, capacity: 4 } : path.endsWith("/join") ? { url: "wss://test.livekit.cloud", token: "secret", connection_id: "connection" } : {});
 container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function mount() { await act(async () => root.render(<MeetVoicePanel postId="post" />)); await settle(); }
it("参加操作まで接続せず、入室はミュート、退出で接続と予約を終了する", async () => {
 await mount(); expect(s.connect).not.toHaveBeenCalled(); await click("通話に参加"); expect(s.connect).toHaveBeenCalledOnce(); expect(s.mic).not.toHaveBeenCalled(); expect(container.textContent).toContain("2/4人");
 await click("マイクをON"); expect(s.mic).toHaveBeenCalledWith(true); await click("退出"); expect(s.disconnect).toHaveBeenCalled(); expect(s.api.mock.calls.some(([path, opts]) => String(path).endsWith("/leave") && opts.body.includes("connection"))).toBe(true);
});
it("マイク拒否でも聞くだけの参加を続けられる", async () => {
 await mount(); await click("通話に参加"); s.mic.mockRejectedValueOnce(new Error("denied")); await click("マイクをON"); expect(container.textContent).toContain("聞くだけの参加は続けられます"); expect(s.disconnect).not.toHaveBeenCalled();
});
it("満員時は入室を送信しない", async () => {
 s.full = true; await mount(); expect([...container.querySelectorAll("button")].find((b) => b.textContent === "満員です")?.disabled).toBe(true); expect(s.connect).not.toHaveBeenCalled();
});
it("再接続を表示し、ログアウトすると切断する", async () => {
 await mount(); await click("通話に参加"); await act(async () => { for (const fn of s.events.get("reconnecting") ?? []) fn(); }); expect(container.textContent).toContain("再接続中"); s.token = null; s.uid = null; await act(async () => root.render(<MeetVoicePanel postId="post" />)); await settle(); expect(s.disconnect).toHaveBeenCalled(); expect(container.textContent).toContain("ログインして参加");
});
it("画面移動後に遅れて届いた参加券も予約を解放する", async () => {
 let resolve!: (data: unknown) => void;
 s.api.mockImplementation((path: string) => path.endsWith("/join") ? new Promise((done) => { resolve = done; }) : Promise.resolve({ enabled: true, full: false, participants: 0, capacity: 4 }));
 await mount(); await click("通話に参加"); await act(async () => root.render(<p>別の画面</p>)); await act(async () => resolve({ connection_id: "late", token: "secret", url: "wss://test.livekit.cloud" })); expect(s.connect).not.toHaveBeenCalled(); expect(s.api.mock.calls.some(([path, opts]) => String(path).endsWith("/leave") && opts.body.includes("late"))).toBe(true);
});
it("未ログインならログインへの入口だけを表示する", async () => { s.token = null; s.uid = null; await mount(); expect(container.querySelector('a[href="/login"]')).toBeTruthy(); expect(s.connect).not.toHaveBeenCalled(); });
it("許可待ちのマイクが画面移動後に返ってきてもトラックを止める", async () => {
 await mount(); await click("通話に参加");
 let resolve!: (value: unknown) => void; const stop = vi.fn();
 s.mic.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
 await click("マイクをON"); await act(async () => root.render(<p>別の画面</p>));
 await act(async () => resolve({ track: { stop } })); expect(stop).toHaveBeenCalledOnce();
});
it("同じ通話の参加者を理由付きで通報できる", async () => {
 await mount(); await click("通話に参加"); await click("通報"); await click("通報する");
 expect(s.api.mock.calls.some(([path, opts]) => path === "/v1/reports" && opts.body.includes('"target_type":"voice_participant"') && opts.body.includes('"target_id":"other"'))).toBe(true);
 expect(container.textContent).toContain("通報済み");
});
