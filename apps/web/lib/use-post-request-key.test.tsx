/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it } from "vitest";
import { usePostRequestKey } from "./use-post-request-key";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function mount() {
  const seen: string[] = [];
  let rerender = () => {};
  function Probe({ tick }: { tick: number }) {
    const key = usePostRequestKey();
    seen.push(key());
    return <span>{tick}</span>;
  }
  const root = createRoot(document.createElement("div"));
  let tick = 0;
  rerender = () => act(() => root.render(<Probe tick={tick++} />));
  rerender();
  return { seen, rerender, unmount: () => act(() => root.unmount()) };
}

// 送り直し(再描画のあと)でも同じキーを使い、フォームを開き直したら別のキーになる。
it("keeps one request key per mounted form", () => {
  const first = mount();
  first.rerender();
  first.rerender();
  expect(new Set(first.seen).size).toBe(1);
  expect(first.seen[0]).toMatch(/^[0-9a-f-]{36}$/);
  first.unmount();

  const second = mount();
  expect(second.seen[0]).not.toBe(first.seen[0]);
  second.unmount();
});
