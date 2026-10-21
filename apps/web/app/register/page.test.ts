/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { describe, expect, it, vi } from "vitest";
import RegisterPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root;
  await act(async () => {
    root = createRoot(div);
    root.render(createElement(RegisterPage));
  });
  return {
    div,
    async unmount() {
      await act(async () => {
        root.unmount();
      });
      div.remove();
    },
  };
}

function buttons(div: HTMLElement) {
  return Array.from(div.querySelectorAll("button"));
}

describe("RegisterPage", () => {
  it("同意するまで登録・Google登録を押せない", async () => {
    const view = await renderPage();
    try {
      const checkbox = view.div.querySelector('input[type="checkbox"]');
      expect(checkbox).not.toBeNull();
      const links = Array.from(view.div.querySelectorAll("a")).map((a) => a.getAttribute("href"));
      expect(links).toContain("/terms");
      expect(links).toContain("/guidelines");
      expect(links).toContain("/privacy");

      const before = buttons(view.div);
      const submit = before.find((b) => b.textContent === "登録");
      const google = before.find((b) => b.textContent?.includes("Googleで登録"));
      expect(submit?.disabled).toBe(true);
      expect(google?.disabled).toBe(true);

      await act(async () => {
        (checkbox as HTMLInputElement).click();
      });

      const after = buttons(view.div);
      expect(after.find((b) => b.textContent === "登録")?.disabled).toBe(false);
      expect(after.find((b) => b.textContent?.includes("Googleで登録"))?.disabled).toBe(false);
    } finally {
      await view.unmount();
    }
  });
});
