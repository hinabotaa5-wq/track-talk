/** @vitest-environment jsdom */
import { act, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { LastTabRestorer } from "./last-tab-restorer";
import { readLastTab, rememberLastTab } from "@/lib/last-tab";

const navigation = vi.hoisted(() => ({ pathname: "/", search: "", router: { replace: vi.fn() } }));
vi.mock("next/navigation", () => ({
  usePathname: () => navigation.pathname,
  useSearchParams: () => new URLSearchParams(navigation.search),
  useRouter: () => navigation.router,
}));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  localStorage.clear();
  navigation.pathname = "/";
  navigation.search = "";
  navigation.router.replace.mockReset();
  window.history.replaceState(null, "", "/");
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
});

async function render(pathname = "/", search = "") {
  navigation.pathname = pathname;
  navigation.search = search;
  await act(async () => root.render(<StrictMode><LastTabRestorer /></StrictMode>));
}

it("新規起動で最後のタブを復元し、復元中は保存値を上書きしない", async () => {
  rememberLastTab("/injury", "tab=data");
  sessionStorage.clear();
  await render();
  expect(navigation.router.replace).toHaveBeenCalledExactlyOnceWith("/injury?tab=data", { scroll: false });
  expect(readLastTab()).toBe("/injury?tab=data");
  await render("/injury", "tab=data");
  await render("/");
  expect(navigation.router.replace).toHaveBeenCalledTimes(1);
  expect(readLastTab()).toBe("/");
});

it("初回はホームを開き、その後のタブ移動を保存する", async () => {
  await render();
  expect(navigation.router.replace).not.toHaveBeenCalled();
  await render("/competitions", "view=international");
  expect(readLastTab()).toBe("/competitions?view=international");
  await render("/competitions/meet-123");
  await render("/login", "next=/memo");
  expect(readLastTab()).toBe("/competitions?view=international");
});

it("ホーム内のタブも記憶し、検索語とページ番号は保存しない", async () => {
  await render("/", "sort=review&type=spike&q=secret&page=3");
  expect(readLastTab()).toBe("/?sort=review&type=spike");
  await act(async () => root.unmount());
  root = createRoot(container);
  await render();
  expect(navigation.router.replace).toHaveBeenCalledWith("/?sort=review&type=spike", { scroll: false });
});

it.each([
  ["/posts/post-123", ""],
  ["/login", "next=/memo"],
  ["/", "sort=new"],
  ["/", "q=race"],
])("直接開いたリンクを優先する: %s?%s", async (pathname, search) => {
  rememberLastTab("/injury", "");
  await render(pathname, search);
  expect(navigation.router.replace).not.toHaveBeenCalled();
});

it("ハッシュ付きのリンクを優先する", async () => {
  rememberLastTab("/memo", "");
  window.history.replaceState(null, "", "/#section");
  await render();
  expect(navigation.router.replace).not.toHaveBeenCalled();
});

it.each(["https://example.com", "//example.com", "/login?next=/memo", "/posts/123", "constructor", "/injury#bad"])("不正な保存値には移動しない: %s", async (saved) => {
  localStorage.setItem("tracktalk:last-tab", saved);
  await render();
  expect(navigation.router.replace).not.toHaveBeenCalled();
  expect(readLastTab()).toBe("/");
});

it("保存が制限されたブラウザでも表示を継続する", async () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
  await render();
  await render("/memo");
  expect(navigation.router.replace).not.toHaveBeenCalled();
});

it("描画前の復元中は二重に移動せず保存値も維持する", async () => {
  rememberLastTab("/memo", "");
  const startupWindow = window as Window & { __tracktalkLastTabRedirect?: boolean };
  startupWindow.__tracktalkLastTabRedirect = true;
  try {
    await render();
    expect(navigation.router.replace).not.toHaveBeenCalled();
    expect(readLastTab()).toBe("/memo");
  } finally {
    delete startupWindow.__tracktalkLastTabRedirect;
  }
});
