/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { SenseNoteSharing } from "./sense-note-sharing";
import type { ContractSenseNote } from "@/lib/sense-notes";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({
  ...(await original<typeof import("@/lib/api")>()),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));

const noteShared: ContractSenseNote = {
  id: "n1", body: "リズムよく走れた", event: "100m", event_label: "100m", day: "2026-10-25",
  created_at: "2026-10-25T03:00:00Z", updated_at: "2026-10-25T03:00:00Z", shared: true,
};
const noteHidden: ContractSenseNote = {
  id: "n2", body: "疲れが残った", event: null, event_label: null, day: "2026-10-24",
  created_at: "2026-10-24T03:00:00Z", updated_at: "2026-10-24T03:00:00Z", shared: false,
};
const startsAt = "2026-10-01T00:00:00+09:00";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-25T03:00:00Z"));
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.useRealTimers();
  setActiveLocale("ja");
});

let renderKey = 0;
async function renderPanel(isCoach: boolean, notes: ContractSenseNote[] = [noteShared, noteHidden]) {
  apiFetch.mockImplementation(async (path: unknown, init?: unknown) => {
    if (typeof path === "string" && path.endsWith("/share")) {
      const body = (init as { body?: string } | undefined)?.body ?? "{}";
      return { shared: (JSON.parse(body) as { shared?: boolean }).shared !== false };
    }
    return { notes, view_closed: false };
  });
  renderKey += 1;
  await act(async () => root.render(
    <SenseNoteSharing key={renderKey} contractId="c1" token="t" isCoach={isCoach} startsAt={startsAt} />,
  ));
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

function dayButton(label: string) {
  return container.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!;
}

it("選手パネル：見出しと注意文が出て、共有日にはチェック、非共有日は淡く、メモなし日は無効", async () => {
  await renderPanel(false);
  expect(container.querySelector("h2")?.textContent).toBe("感覚メモの共有");
  expect(container.textContent).toContain("チェックを外すと、コーチはそのメモを見られなくなります");
  const shared = dayButton("2026年10月25日、感覚メモあり 共有");
  expect(shared.disabled).toBe(false);
  expect(shared.getAttribute("aria-pressed")).toBe("true");
  expect(shared.querySelector("svg")).not.toBeNull();
  const hidden = dayButton("2026年10月24日、感覚メモあり 非表示");
  expect(hidden.disabled).toBe(false);
  expect(hidden.getAttribute("aria-pressed")).toBe("false");
  expect(hidden.querySelector("svg")).toBeNull();
  expect(hidden.className).toContain("opacity-70");
  expect(dayButton("2026年10月23日、感覚メモなし").disabled).toBe(true);
});

it("選手のタップ：共有の付け外しでPOSTし、失敗したら元に戻してエラーを出す", async () => {
  await renderPanel(false);
  await act(async () => dayButton("2026年10月25日、感覚メモあり 共有").click());
  expect(apiFetch).toHaveBeenLastCalledWith(
    "/v1/coaching/contracts/c1/sense-notes/n1/share",
    { method: "POST", token: "t", body: JSON.stringify({ shared: false }) },
  );
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  expect(dayButton("2026年10月25日、感覚メモあり 非表示")).not.toBeNull();
  expect(container.querySelector('[role="dialog"]')).toBeNull();
  apiFetch.mockRejectedValueOnce(new Error("down"));
  await act(async () => dayButton("2026年10月25日、感覚メモあり 非表示").click());
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  expect(dayButton("2026年10月25日、感覚メモあり 非表示")).not.toBeNull();
  expect(container.textContent).toContain("送信に失敗しました");
});

it("選手の長押し：プレビューが開き、付け外しは発火しない。通常タップでは開かない", async () => {
  await renderPanel(false);
  const posts = () => apiFetch.mock.calls.filter((c) => String(c[0]).endsWith("/share")).length;
  expect(posts()).toBe(0);
  await act(async () => {
    dayButton("2026年10月25日、感覚メモあり 共有").dispatchEvent(new Event("pointerdown", { bubbles: true }));
  });
  await act(async () => {
    vi.advanceTimersByTime(500);
  });
  const dialog = container.querySelector('[role="dialog"]')!;
  expect(dialog.textContent).toContain("リズムよく走れた");
  expect(dialog.textContent).toContain("100m");
  expect(dialog.textContent).not.toContain("削除");
  await act(async () => {
    dayButton("2026年10月25日、感覚メモあり 共有").dispatchEvent(new Event("pointerup", { bubbles: true }));
    dayButton("2026年10月25日、感覚メモあり 共有").click();
  });
  expect(posts()).toBe(0);
  expect(container.querySelector('[role="dialog"]')).not.toBeNull();
  await act(async () => {
    container.querySelector<HTMLButtonElement>('button[aria-label="詳細を閉じる"]')!.click();
  });
  expect(container.querySelector('[role="dialog"]')).toBeNull();
});

it("月移動はwindow開始月から今月まで", async () => {
  await renderPanel(false);
  expect(container.querySelector<HTMLButtonElement>('button[aria-label="次の月"]')!.disabled).toBe(true);
  await act(async () => {
    container.querySelector<HTMLButtonElement>('button[aria-label="前の月"]')!.click();
  });
  expect(container.textContent).toContain("9月");
  expect(container.textContent).toContain("この月の感覚メモはありません。");
  expect(container.querySelector<HTMLButtonElement>('button[aria-label="前の月"]')!.disabled).toBe(true);
  await act(async () => {
    container.querySelector<HTMLButtonElement>('button[aria-label="次の月"]')!.click();
  });
  expect(container.textContent).toContain("10月");
});

it("コーチパネル：共有中だけドット、タップでプレビュー、操作要素なし、0件で空文言", async () => {
  await renderPanel(true, [noteShared]);
  expect(container.querySelector("h2")?.textContent).toBe("選手の感覚メモ");
  expect(container.textContent).not.toContain("チェックを外すと");
  const shared = dayButton("2026年10月25日、感覚メモあり");
  expect(shared.querySelector(".bg-sky-300")).not.toBeNull();
  expect(shared.hasAttribute("aria-pressed")).toBe(false);
  expect(dayButton("2026年10月24日、感覚メモなし").disabled).toBe(true);
  await act(async () => shared.click());
  await act(async () => {
    await Promise.resolve();
  });
  expect(container.querySelector('[role="dialog"]')?.textContent).toContain("リズムよく走れた");
  expect(apiFetch.mock.calls.filter((c) => String(c[0]).endsWith("/share"))).toHaveLength(0);
  await renderPanel(true, []);
  expect(container.textContent).toContain("共有されている感覚メモはありません");
});

it("英語では見出しと注意文と空文言が英語になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await renderPanel(false);
  expect(container.querySelector("h2")?.textContent).toBe("Sharing sense notes");
  expect(container.textContent).toContain("If you uncheck a note, the coach will no longer be able to see it.");
  await renderPanel(true, []);
  expect(container.querySelector("h2")?.textContent).toBe("Athlete's sense notes");
  expect(container.textContent).toContain("No sense notes are being shared.");
});
