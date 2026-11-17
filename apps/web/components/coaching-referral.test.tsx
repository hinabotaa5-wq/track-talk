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

it("終了した紹介IDは赤い取り消し線で薄く表示し、IDの文字列はそのまま残す（本体 #920 A1・A2）", async () => {
  const strong = await renderCard({ code: "CWN8X2", active: false, friend_count: 1, granted_at: null, expires_at: null });
  expect(strong.textContent).toBe("CWN8X2");
  for (const c of [...endedClasses, "decoration-2", "font-mono", "tracking-wider"]) expect(strong.classList).toContain(c);
  expect(copyButton()).toBeUndefined();
  expect(container.textContent).toContain(endedNotice);
});

it("特典付与済みで終了した周でも同じ見た目にし、付与済みの表示は残す（本体 #920 A3）", async () => {
  const strong = await renderCard({ code: "K7M3QX", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: "2027-05-14T09:00:00Z" });
  expect(strong.textContent).toBe("K7M3QX");
  for (const c of [...endedClasses, "font-mono", "tracking-wider"]) expect(strong.classList).toContain(c);
  expect(container.textContent).toContain("無料チケットをプレゼントしました");
  expect(container.textContent).toContain(endedNotice);
  expect(copyButton()).toBeUndefined();
});

it("有効な紹介IDには取り消し線も薄い色も付けず、コピーボタンを出す（本体 #920 A4）", async () => {
  const strong = await renderCard({ code: "K7M3QX", active: true, friend_count: 1, granted_at: null, expires_at: null });
  expect(strong.textContent).toBe("K7M3QX");
  for (const c of endedClasses) expect(strong.classList).not.toContain(c);
  expect(strong.className).toBe("font-mono tracking-wider");
  expect(copyButton()).toBeDefined();
  expect(container.textContent).not.toContain(endedNotice);
});
