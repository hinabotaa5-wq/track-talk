// @vitest-environment jsdom
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { clearLinkPending, markLinkPending, useIsLinkPending } from "./pending-link";
import { pendingLinkStartupScript } from "./pending-link-startup";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function Probe({ id }: { id: string }) {
  return <span>{useIsLinkPending(id) ? "pending" : "idle"}</span>;
}

afterEach(() => {
  clearLinkPending();
  document.body.innerHTML = "";
  window.history.replaceState(null, "", "/");
});

describe("タップしたリンクのクルクル", () => {
  it("タップしたリンクだけを、URL が変わるまで回す", async () => {
    const container = document.createElement("div");
    document.body.append(container);
    const root = createRoot(container);
    await act(async () => root.render(<><Probe id="a" /><Probe id="b" /></>));
    await act(async () => markLinkPending("a", "/memo"));
    expect(container.textContent).toBe("pendingidle");
    await act(async () => clearLinkPending());
    expect(container.textContent).toBe("idleidle");
    await act(async () => root.unmount());
  });

  it("いまの画面と同じ URL へのタップでは回さない", async () => {
    window.history.replaceState(null, "", "/memo");
    const container = document.createElement("div");
    document.body.append(container);
    const root = createRoot(container);
    await act(async () => root.render(<Probe id="a" />));
    await act(async () => markLinkPending("a", "/memo"));
    expect(container.textContent).toBe("idle");
    await act(async () => root.unmount());
  });

  it("React が動く前のタップには印を付け、動いたあとや同じ画面へのタップには付けない", () => {
    window.history.replaceState(null, "", "/");
    new Function(pendingLinkStartupScript)();
    document.body.innerHTML = '<a data-pending-link href="/memo"><span>メモ</span></a><a data-pending-link href="/">ホーム</a>';
    const [memo, home] = Array.from(document.querySelectorAll("a"));
    const click = (el: Element) => el.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 }));
    // 印を付けたあとで既定動作を止め、jsdom にページ移動をさせない。
    window.addEventListener("click", (event) => event.preventDefault(), { once: false });
    click(home);
    expect(home.hasAttribute("data-boot-pending")).toBe(false);
    click(memo.querySelector("span")!);
    expect(memo.hasAttribute("data-boot-pending")).toBe(true);
    clearLinkPending();
    expect(memo.hasAttribute("data-boot-pending")).toBe(false);
    (window as Window & { __tracktalkHydrated?: boolean }).__tracktalkHydrated = true;
    click(memo);
    expect(memo.hasAttribute("data-boot-pending")).toBe(false);
  });
});
