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

const rows = [{ label: "内容", value: "1回プラン" }];
async function render(props: { pending?: boolean; pendingLabel?: string; onConfirm?: () => void; onBack?: () => void }) {
  await act(async () => root.render(
    <PurchaseConfirm rows={rows} confirmLabel="確定してお支払いへ" onConfirm={props.onConfirm ?? (() => {})} onBack={props.onBack ?? (() => {})}
      pending={props.pending} pendingLabel={props.pendingLabel} />,
  ));
}
const waiting = () => container.querySelector('[role="status"]');
const buttons = () => [...container.querySelectorAll("button")];

// 本体 #917 A1
it("pending のあいだはストップウォッチと渡した文言を出し、pending でなければ出さない", async () => {
  await render({ pending: true, pendingLabel: "お支払い画面へ移動しています" });
  expect(waiting()?.querySelector("svg.screen-loader-watch")).not.toBeNull();
  expect(waiting()?.textContent).toBe("お支払い画面へ移動しています");
  // ボタンの下に出す。
  expect(buttons()[1].compareDocumentPosition(waiting()!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

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
  expect(en["お支払い画面へ移動しています"]).toBe("Taking you to the payment page");
});
