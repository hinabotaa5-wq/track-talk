/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { checkReferralInput, LoadedReferralCard, ReferralInput, type ReferralWarning } from "./coaching-referral";

const mocks = vi.hoisted(() => ({ api: vi.fn() }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  Object.defineProperty(navigator, "share", { configurable: true, value: undefined });
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn() } });
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

const note = "紹介IDを使って購入すると、この購入ではあなたの紹介IDは発行されません。";

async function renderInput(code: string, referral: unknown, warning: ReferralWarning = false) {
  mocks.api.mockResolvedValue({ referral });
  await act(async () => root.render(<ReferralInput token="test" code={code} onChange={() => {}} warning={warning} />));
}

it.each([
  { name: "IDを持たない人", referral: null },
  { name: "周が終了した人", referral: { code: "K7M3QX", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: null } },
])("有効な紹介IDを持たない人が紹介IDを入れると、自分のIDが発行されない旨を添える（$name）", async ({ referral }) => {
  await renderInput("ABCDEF", referral);
  expect(container.textContent).toContain(note);
});

it.each<{ name: string; code: string; referral: unknown; warning: ReferralWarning }>([
  { name: "空欄", code: "", referral: null, warning: false },
  { name: "有効なIDを持つ人", code: "ABCDEF", referral: { code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null }, warning: false },
  { name: "使えないIDの警告中", code: "ABCDEF", referral: null, warning: "invalid" },
])("注記を出さない（$name）", async ({ code, referral, warning }) => {
  await renderInput(code, referral, warning);
  expect(container.textContent).not.toContain(note);
});

it("紹介者にすでに数えられている購入者には、その旨を伝える（本体 #686）", async () => {
  mocks.api.mockResolvedValue({ valid: false, already_counted: true });
  expect(await checkReferralInput("ABCDEF", "test")).toBe("counted");
  mocks.api.mockResolvedValue({ valid: false, already_counted: false });
  expect(await checkReferralInput("ABCDEF", "test")).toBe("invalid");
  expect(await checkReferralInput("  ", "test")).toBe("ok");
  await renderInput("", null, "counted");
  expect(container.textContent).toContain("この紹介IDの紹介者には、あなたの購入がすでに数えられています。");
});

it("入力欄のラベルは「紹介ID」だけで、「友達からもらった場合」を含まない", async () => {
  await renderInput("", null);
  const label = container.querySelector("label");
  expect(label?.firstChild?.textContent).toBe("紹介ID");
  expect(container.textContent).not.toContain("友達からもらった場合");
});

const endedNotice = "このIDは終了しました。紹介IDを使わずに購入すると、新しい紹介IDが発行されます。";
const endedClasses = ["line-through", "decoration-red-500", "text-slate-500"];

async function renderCard(referral: unknown) {
  mocks.api.mockResolvedValue({ referral });
  await act(async () => root.render(<LoadedReferralCard token="test" />));
  const strong = container.querySelector("strong");
  if (!strong) throw new Error("紹介IDが描画されていない");
  return strong;
}

const copyButton = () => [...container.querySelectorAll("button")].find((b) => b.textContent === "コピー");
const shareButton = () => [...container.querySelectorAll("button")].find((b) => b.textContent === "共有");
const referralShareText = "TrackTalkでコーチにフォーム添削してもらえるよ。購入のときに紹介ID「K7M3QX」を入れてね。";
const referralShareUrl = "https://track-talk.netlify.app/referral";

it("終了した紹介IDは赤い取り消し線で薄く表示し、IDの文字列はそのまま残す（本体 #920 A1・A2）", async () => {
  const strong = await renderCard({ code: "CWN8X2", active: false, friend_count: 1, granted_at: null, expires_at: null });
  expect(strong.textContent).toBe("CWN8X2");
  for (const c of [...endedClasses, "decoration-2", "font-mono", "tracking-wider"]) expect(strong.classList).toContain(c);
  expect(copyButton()).toBeUndefined();
  expect(shareButton()).toBeUndefined();
  expect(container.textContent).toContain(endedNotice);
});

it("特典付与済みで終了した周でも同じ見た目にし、付与済みの表示は残す（本体 #920 A3）", async () => {
  const strong = await renderCard({ code: "K7M3QX", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: "2027-05-14T09:00:00Z" });
  expect(strong.textContent).toBe("K7M3QX");
  for (const c of [...endedClasses, "font-mono", "tracking-wider"]) expect(strong.classList).toContain(c);
  expect(container.textContent).toContain("無料チケットをプレゼントしました");
  expect(container.textContent).toContain(endedNotice);
  expect(copyButton()).toBeUndefined();
  expect(shareButton()).toBeUndefined();
});

it("有効な紹介IDには取り消し線も薄い色も付けず、コピーボタンを出す（本体 #920 A4）", async () => {
  const strong = await renderCard({ code: "K7M3QX", active: true, friend_count: 1, granted_at: null, expires_at: null });
  expect(strong.textContent).toBe("K7M3QX");
  for (const c of endedClasses) expect(strong.classList).not.toContain(c);
  expect(strong.className).toBe("font-mono tracking-wider");
  expect(copyButton()).toBeDefined();
  expect(shareButton()).toBeDefined();
  expect(copyButton()?.nextElementSibling).toBe(shareButton());
  expect(container.textContent).not.toContain(endedNotice);
});

it("compact は見出し・ID・コピー・共有・状態だけでコピー結果も表示する", async () => {
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  mocks.api.mockResolvedValue({ referral: { code: "ABC123", active: true, friend_count: 1, granted_at: null, expires_at: null } });
  await act(async () => root.render(<LoadedReferralCard token="test" variant="compact" />));
  expect(container.querySelector("h2")?.textContent).toBe("友達2人に紹介すると、1回分(1,000円)無料！");
  expect(container.querySelector("h2")?.classList).toContain("text-sm");
  expect(container.querySelector("section")?.classList).toContain("p-3");
  expect(container.textContent).toContain("あなたの紹介ID ABC123");
  expect(container.textContent).toContain("あと1人で無料チケット");
  expect(container.textContent).not.toContain("購入すると、紹介IDが発行されます。");
  expect(shareButton()).toBeDefined();
  await act(async () => copyButton()!.click());
  expect(writeText).toHaveBeenCalledWith("ABC123");
  expect(container.querySelector('[role="status"]')?.textContent).toBe("コピーしました");
});

it.each(["full", "compact"] as const)("%s の終了IDは取り消し線・グレー・コピーなしで、付与詳細はfullだけに出る", async (variant) => {
  mocks.api.mockResolvedValue({ referral: { code: "ABC123", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: "2027-05-14T09:00:00Z" } });
  await act(async () => root.render(<LoadedReferralCard token="test" variant={variant} />));
  for (const c of endedClasses) expect(container.querySelector("strong")?.classList).toContain(c);
  expect(copyButton()).toBeUndefined();
  expect(shareButton()).toBeUndefined();
  expect(container.textContent).toContain("無料チケットをプレゼントしました");
  for (const text of ["購入すると、紹介IDが発行されます。", "付与日", "有効期限", "特典チケット：", endedNotice]) {
    expect(container.textContent?.includes(text)).toBe(variant === "full");
  }
});

it("共有ボタンは紹介文とURLをWeb Share APIへ渡し、成功時はコピーも状態表示もしない", async () => {
  const share = vi.fn(async () => {});
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "share", { configurable: true, value: share });
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  await renderCard({ code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null });
  await act(async () => shareButton()!.click());
  expect(share).toHaveBeenCalledTimes(1);
  expect(share).toHaveBeenCalledWith({ text: referralShareText, url: referralShareUrl });
  expect(writeText).not.toHaveBeenCalled();
  expect(container.querySelector('[role="status"]')).toBeNull();
});

it("共有シートを閉じたときはコピーも状態表示もしない", async () => {
  const share = vi.fn(async () => { throw new DOMException("cancelled", "AbortError"); });
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "share", { configurable: true, value: share });
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  await renderCard({ code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null });
  await act(async () => shareButton()!.click());
  expect(writeText).not.toHaveBeenCalled();
  expect(container.querySelector('[role="status"]')).toBeNull();
});

it("Web Share APIがないときは共有文とURLをコピーして状態を表示する", async () => {
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  await renderCard({ code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null });
  await act(async () => shareButton()!.click());
  expect(writeText).toHaveBeenCalledWith(`${referralShareText}\n${referralShareUrl}`);
  expect(container.querySelector('[role="status"]')?.textContent).toBe("共有文をコピーしました");
});

it("共有がAbortError以外で失敗したときは共有文とURLをコピーする", async () => {
  const share = vi.fn(async () => { throw new Error("unavailable"); });
  const writeText = vi.fn(async () => {});
  Object.defineProperty(navigator, "share", { configurable: true, value: share });
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  await renderCard({ code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null });
  await act(async () => shareButton()!.click());
  expect(writeText).toHaveBeenCalledWith(`${referralShareText}\n${referralShareUrl}`);
  expect(container.querySelector('[role="status"]')?.textContent).toBe("共有文をコピーしました");
});

it("共有文のコピーにも失敗したときは失敗を表示する", async () => {
  const writeText = vi.fn(async () => { throw new Error("denied"); });
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  await renderCard({ code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null });
  await act(async () => shareButton()!.click());
  expect(container.querySelector('[role="status"]')?.textContent).toBe("共有できませんでした");
});
