/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MeetRoomContent } from "./meet-room-content";

const apiFetch = vi.fn();
vi.mock("@/lib/api", () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }));
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("公式サイト、タイムテーブル、生配信の順で表示し、自動再生しない", async () => {
  apiFetch.mockResolvedValue({ official_url: "https://example.com/", timetable_url: "https://example.com/time.pdf", youtube_video_id: "abcdefghijk" });
  await act(async () => root.render(<MeetRoomContent postId="meet" body="大会の実況" />));
  expect([...container.querySelectorAll("a, iframe")].map((el) => el.tagName)).toEqual(["A", "A", "IFRAME", "A"]);
  expect(container.querySelectorAll("a")[1].href).toBe("https://example.com/time.pdf");
  const src = new URL(container.querySelector("iframe")!.src);
  expect(src.pathname).toBe("/embed/abcdefghijk");
  expect(src.searchParams.get("autoplay")).toBeNull();
});

it("取得に失敗しても既存の公式サイトをリンクとして残す", async () => {
  apiFetch.mockRejectedValue(new Error("offline"));
  await act(async () => root.render(<MeetRoomContent postId="meet" body={"実況本文\n公式サイト: https://example.com/"} />));
  expect(container.textContent).toContain("実況本文");
  expect(container.querySelector("a")?.href).toBe("https://example.com/");
  expect(container.querySelector("iframe")).toBeNull();
});

it("不正なリンクや動画IDを表示しない", async () => {
  apiFetch.mockResolvedValue({ official_url: "javascript:alert(1)", timetable_url: "https://user:password@example.com/", youtube_video_id: "bad/id" });
  await act(async () => root.render(<MeetRoomContent postId="meet" body="実況本文" />));
  expect(container.querySelector("a, iframe")).toBeNull();
});
