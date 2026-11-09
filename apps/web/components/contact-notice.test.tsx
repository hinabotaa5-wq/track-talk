/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ContactNotice, useSolicitationGuard } from "./contact-notice";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

function Form({ text, onSend }: { text: string; onSend: () => void }) {
  const guard = useSolicitationGuard();
  return <form onSubmit={(e) => { e.preventDefault(); if (guard.allow(text)) onSend(); }}><ContactNotice />{guard.warning}<button type="submit">送る</button></form>;
}
const press = () => act(async () => { container.querySelector("button")!.click(); });

it("注意文は常に出る。勧誘の言葉は1回目だけ止めて警告し、もう一度押すと送る", async () => {
  const onSend = vi.fn();
  await act(async () => root.render(<Form text="LINE で話しましょう" onSend={onSend} />));
  expect(container.textContent).toContain("規約で禁止されています");
  await press();
  expect(onSend).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("もう一度押すとそのまま送ります");
  await press();
  expect(onSend).toHaveBeenCalledTimes(1);
});

it("ふつうの文はそのまま送る", async () => {
  const onSend = vi.fn();
  await act(async () => root.render(<Form text="スタートラインの位置を直しましょう" onSend={onSend} />));
  await press();
  expect(onSend).toHaveBeenCalledTimes(1);
  expect(container.querySelector('[role="alert"]')).toBeNull();
});
