/** @vitest-environment jsdom */
import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

// motion のピルかどうかを DOM で見分けられるよう、motion.span だけを印つきの span に差し替える。
vi.mock("motion/react", () => ({
  motion: {
    span: (props: Record<string, unknown>) => (
      <span data-motion-pill={String(props.layoutId)} className={props.className as string} aria-hidden={props["aria-hidden"] as boolean} />
    ),
  },
}));
const { ViewSwitch } = await import("./view-switch");

let root: Root;
let container: HTMLDivElement;
let frameCallback: FrameRequestCallback | null;

function Harness({ initialPillStatic = false }: { initialPillStatic?: boolean }) {
  const [current, setCurrent] = useState<"video" | "support">("video");
  return (
    <ViewSwitch
      label="切り替え"
      layoutId="test-pill"
      views={[
        { id: "video", label: "スタンダード" },
        { id: "support", label: "継続プラン" },
      ] as const}
      current={current}
      onSelect={setCurrent}
      initialPillStatic={initialPillStatic}
    />
  );
}

beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  frameCallback = null;
  vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
    frameCallback = cb;
    return 1;
  });
  vi.stubGlobal("cancelAnimationFrame", () => {});
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

const pill = () => container.querySelector("span.bg-blue-600");
const render = async (props: { initialPillStatic?: boolean } = {}) => {
  await act(async () => root.render(<Harness {...props} />));
};

it("initialPillStatic では最初のピルはアニメーションなしで置かれ、1フレーム後から滑るピルに代わる", async () => {
  await render({ initialPillStatic: true });
  // 最初の表示では motion のピルではなく、最終位置に置いただけの span である。
  expect(pill()).not.toBeNull();
  expect(pill()?.hasAttribute("data-motion-pill")).toBe(false);
  expect(pill()?.parentElement?.textContent).toContain("スタンダード");
  await act(async () => frameCallback?.(0));
  expect(pill()?.getAttribute("data-motion-pill")).toBe("test-pill");
  expect(pill()?.parentElement?.textContent).toContain("スタンダード");
});

it("initialPillStatic のピルはフレーム後、切り替えで相手側へ移る", async () => {
  await render({ initialPillStatic: true });
  await act(async () => frameCallback?.(0));
  const buttons = [...container.querySelectorAll("button")];
  await act(async () => buttons[1].click());
  expect(pill()?.getAttribute("data-motion-pill")).toBe("test-pill");
  expect(pill()?.parentElement?.textContent).toContain("継続プラン");
  expect(buttons[1].getAttribute("aria-pressed")).toBe("true");
});

it("initialPillStatic を指定しない画面は従来どおり最初から motion のピルである", async () => {
  await render();
  expect(pill()?.getAttribute("data-motion-pill")).toBe("test-pill");
});
