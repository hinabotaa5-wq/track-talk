/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { PurchaseConfirm } from "./purchase-confirm";

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

const rows = [{ label: "内容", value: "スタンダード" }];
async function render(props: { pending?: boolean; pendingLabel?: string; onConfirm?: () => void; onBack?: () => void }) {
  await act(async () => root.render(
    <PurchaseConfirm rows={rows} confirmLabel="確定してお支払いへ" onConfirm={props.onConfirm ?? (() => {})} onBack={props.onBack ?? (() => {})}
      pending={props.pending} pendingLabel={props.pendingLabel} />,
  ));
}
const waiting = () => container.querySelector('[role="status"]');
const buttons = () => [...container.querySelectorAll("button")];
const background = () => {
  const section = container.querySelector("section");
  if (!section) return [];
  return [...section.children].filter((el) => el !== waiting()) as HTMLElement[];
};

// 本体 #917 A1
it("pending のあいだはストップウォッチと渡した文言を出し、pending でなければ出さない", async () => {
  await render({ pending: true, pendingLabel: "お支払い画面へ移動しています" });
  expect(waiting()?.querySelector("svg.screen-loader-watch")).not.toBeNull();
  expect(waiting()?.textContent).toBe("お支払い画面へ移動しています");
  expect(waiting()?.getAttribute("aria-busy")).toBe("true");
  for (const cls of ["fixed", "inset-0", "z-[110]", "pointer-events-auto", "bg-slate-950/70", "backdrop-blur-md", "justify-center"]) {
    expect(waiting()?.classList.contains(cls)).toBe(true);
  }

  await render({ pending: false, pendingLabel: "お支払い画面へ移動しています" });
  expect(waiting()).toBeNull();
  expect(container.querySelector("svg.screen-loader-watch")).toBeNull();
  expect(container.textContent).not.toContain("お支払い画面へ移動しています");
});

// 本体 #917 A2
it("pendingLabel を渡さないときは「処理しています」を出す", async () => {
  await render({ pending: true });
  expect(waiting()?.textContent).toBe("処理しています");
  expect(waiting()?.querySelector("svg.screen-loader-watch")).not.toBeNull();
});

// 本体 #917 A3
it("pending のあいだは確定・戻るとも押せず、確定は発火しない", async () => {
  const onConfirm = vi.fn();
  const onBack = vi.fn();
  await render({ pending: true, onConfirm, onBack });
  expect(buttons().map((b) => b.disabled)).toEqual([true, true]);
  await act(async () => { for (const b of buttons()) b.click(); });
  expect(onConfirm).not.toHaveBeenCalled();
  expect(onBack).not.toHaveBeenCalled();
});

it("待ちの文言は英語表示では英訳で出る", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render({ pending: true });
  expect(waiting()?.textContent).toBe("Processing");
  await render({ pending: true, pendingLabel: en["お支払い画面へ移動しています"] });
  expect(waiting()?.textContent).toBe("Taking you to the payment page");
});

it("待っている間はキーボードのフォーカスを留め、解除すると元に戻す", async () => {
  const back = document.createElement("button");
  document.body.prepend(back);
  try {
    back.focus();
    await render({ pending: true });
    expect(document.activeElement).toBe(waiting());
    const tab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    await act(async () => waiting()!.dispatchEvent(tab));
    expect(tab.defaultPrevented).toBe(true);
    await render({ pending: false });
    expect(document.activeElement).toBe(back);
    expect(buttons().map((b) => b.disabled)).toEqual([false, false]);
  } finally { back.remove(); }
});

it("pending のあいだは背面（オーバーレイ以外の部分）が inert になり、終わると外れる", async () => {
  await render({ pending: false });
  expect(background().length).toBeGreaterThan(0);
  for (const el of background()) expect(el.inert === true).toBe(false);

  await render({ pending: true });
  for (const el of background()) expect(el.inert === true).toBe(true);
  expect((waiting() as HTMLElement).inert === true).toBe(false);

  await render({ pending: false });
  for (const el of background()) expect(el.inert === true).toBe(false);
});

it("pending のままアンマウントしても inert が残らない", async () => {
  await render({ pending: true });
  const elements = background();
  expect(elements.length).toBeGreaterThan(0);
  for (const el of elements) expect(el.inert === true).toBe(true);
  await act(async () => root.unmount());
  for (const el of elements) expect(el.inert === true).toBe(false);
  root = createRoot(container);
});
