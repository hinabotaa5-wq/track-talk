/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const apiFetch = vi.fn();
const routerReplace = vi.fn();
const auth = { token: "tok" as string | null };
const nav = { tab: null as string | null, pathname: "/coaching" };
const flags = { coaching: false, list: false };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: routerReplace }),
  usePathname: () => nav.pathname,
  useSearchParams: () => ({ get: (key: string) => (key === "tab" ? nav.tab : null) }),
}));
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => <a href={href} {...props}>{children}</a>,
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token }) }));
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
  get coachingCoachListEnabled() { return flags.list; },
}));
vi.mock("@/components/coach-apply-link", () => ({ CoachApplyLink: () => <a href="/coach/apply">コーチに応募する</a> }));
vi.mock("@/components/coach-photo", () => ({ CoachAvatar: ({ name }: { name: string }) => <span>{name}の顔写真</span> }));
const { default: CoachingPage } = await import("./page");

const coach = {
  id: "coach-1", nickname: "テストコーチ", photo_url: "", tier: "coach", events: ["100m"],
  best_record: "", achievements: "", years: 1, sns_urls: [], rating_count: 0, rating_average: 0,
  service_types: ["video_review"],
  plans: [{ id: "plan-1", title: "公開中の動画添削", kind: "single", description: "", duration_days: 7,
    price_yen: 2000, capacity: 3, published: true, service_type: "video_review",
    video_reviews: 1, menu_updates: null, reply_formats: [], start_on_first_video: false }],
};

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); routerReplace.mockReset();
  apiFetch.mockImplementation((path: string) => {
    if (path === "/v1/coaches") return Promise.resolve({ coaches: [coach] });
    if (path === "/v1/coaches/support-plan") return Promise.resolve({ coaches: [] });
    if (path === "/v1/coach/me") return Promise.resolve({ application: null });
    return Promise.reject(new Error(`unexpected ${path}`));
  });
  auth.token = "tok"; nav.tab = null; nav.pathname = "/coaching";
  flags.coaching = false; flags.list = false;
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = async () => { await act(async () => root.render(<CoachingPage />)); await act(async () => { await Promise.resolve(); }); };
const switchTo = async (label: string) => {
  const el = [...container.querySelectorAll("a")].find((a) => a.textContent === label);
  await act(async () => { (el as HTMLAnchorElement).click(); });
};

describe("コーチタブの着地 (/coaching)", () => {
  it("一覧フラグが閉じている間は準備中だけを出し、一覧を取得しない", async () => {
    flags.coaching = true;
    await render();
    expect(container.textContent).toContain("ただいま準備中です");
    expect(apiFetch).not.toHaveBeenCalledWith("/v1/coaches");
    expect(container.textContent).not.toContain("テストコーチ");
  });

  it("両フラグを開けると /coaches と同じ一覧を出し、ハブのカードメニューは出さない", async () => {
    flags.coaching = true; flags.list = true;
    await render();
    expect(container.textContent).toContain("テストコーチ");
    // ハブ時代のChoice・利用中の方は出さない。
    for (const word of ["コーチ専用画面", "利用中の方", "コーチングは準備中です", "登録しているコーチを見る"]) {
      expect(container.textContent).not.toContain(word);
    }
  });

  it("見出し「コーチ一覧」と説明文を出さない", async () => {
    flags.coaching = true; flags.list = true;
    await render();
    expect(container.querySelector("h1")).toBeNull();
    expect(container.textContent).not.toContain("コーチ一覧");
    expect(container.textContent).not.toContain("上のタブで、動画添削の指名とサポートプランを切り替えられます。");
  });

  it("切り替えは今のルートを保って tab だけを変える", async () => {
    flags.coaching = true; flags.list = true;
    await render();
    await switchTo("サポートプラン");
    expect(routerReplace).toHaveBeenCalledWith("/coaching?tab=support");
  });

  it("?tab=support の直接表示でサポート側を開く", async () => {
    flags.coaching = true; flags.list = true; nav.tab = "support";
    await render();
    expect(container.textContent).toContain("サポートプランは準備中です");
  });

  it("メタ説明はハブの準備中文ではなく一覧の実態に合わせる", async () => {
    const coaching = (await import("./page")).metadata;
    const coaches = (await import("../coaches/page")).metadata;
    expect(coaching.description).toBe(coaches.description);
    expect(coaching.description).not.toMatch(/準備中|募集/);
  });
});
