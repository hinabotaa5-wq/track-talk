/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { ApiRequestError } from "@/lib/api";
import { rememberCoachDetailPreview } from "@/lib/coach-detail-preview";
import type { PublicCoach } from "@/lib/types";
import { CoachList } from "./coach-list";
import CoachDetailPage from "./[id]/page";

const apiFetch = vi.fn();
const routerBack = vi.fn();
const routerPush = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "coach-1" }), useSearchParams: () => new URLSearchParams(), useRouter: () => ({ back: routerBack, push: routerPush }) }));
vi.mock("next/link", () => ({ default: ({ href, children, onNavigate, ...props }: { href: string; children: ReactNode; onNavigate?: () => void }) => <a href={href} onClick={(event) => { event.preventDefault(); onNavigate?.(); }} {...props}>{children}</a> }));
vi.mock("@/components/coach-photo", () => ({ CoachAvatar: ({ name }: { name: string }) => <span>{name}の顔写真</span> }));
vi.mock("@/components/coaching-plans", () => ({ PublicPlanList: ({ plans }: { plans: { title: string }[] }) => <div>{plans.map((plan) => plan.title).join("、")}</div> }));

const coach: PublicCoach = {
  id: "coach-1", nickname: "テストコーチ", photo_url: "", tier: "coach", events: ["100m"],
  best_record: "10秒台", achievements: "全国大会出場", years: 10, sns_urls: ["https://www.instagram.com/testcoach/", "@tracktalk.office", "javascript:alert(1)"], hitokoto: "よろしくお願いします", rating_count: 0, rating_average: 0,
  service_types: ["video_review"],
  plans: [{ id: "plan-1", title: "公開中の動画添削", kind: "single", description: "説明", duration_days: 7,
    price_yen: 2000, capacity: 3, published: true, service_type: "video_review",
    video_reviews: 1, menu_updates: null, reply_formats: [], start_on_first_video: false }],
};
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
  rememberCoachDetailPreview(coach, 0);
  routerBack.mockReset();
  routerPush.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

it("一覧は2列で各コーチの詳細ページへリンクし、プランは一覧に出さない", async () => {
  apiFetch.mockResolvedValue({ coaches: [coach, { ...coach, id: "coach-2", nickname: "別のコーチ" }] });
  await act(async () => root.render(<CoachList />));
  expect(container.querySelector("ul")?.classList.contains("grid-cols-2")).toBe(true);
  expect([...container.querySelectorAll("a")].map((a) => a.getAttribute("href"))).toEqual(["/coaches/coach-1", "/coaches/coach-2"]);
  expect(container.textContent).not.toContain("公開中の動画添削");
  expect(container.textContent).toContain("よろしくお願いします");
});

it("詳細URLを直接開いても対象コーチのプロフィールと公開プランを取得する", async () => {
  apiFetch.mockResolvedValue(coach);
  await act(async () => root.render(<CoachDetailPage />));
  expect(apiFetch.mock.calls).toEqual([["/v1/coaches/coach-1"]]);
  expect(container.querySelector("h1")?.textContent).toBe("テストコーチ");
  expect(container.textContent).toContain("全国大会出場");
  expect(container.textContent).toContain("成績や指導経験");
  expect(container.textContent).not.toContain("公開中の動画添削");
  const video = [...container.querySelectorAll("button")].find((button) => button.textContent?.includes("動画添削"));
  expect(video).toBeUndefined();
  expect(container.textContent).not.toContain("公開中の動画添削");
  const sns = [...container.querySelectorAll("a")].find((a) => a.getAttribute("href") === "https://www.instagram.com/testcoach/");
  expect(sns?.getAttribute("target")).toBe("_blank");
  expect(container.textContent).toContain("よろしくお願いします");
  expect(container.textContent).toContain("SNS・ホームページ");
  expect(container.textContent).not.toContain("SNS・ホームページ・ひとこと");
  for (const entry of ["@tracktalk.office", "javascript:alert(1)"]) {
    expect(container.textContent).toContain(entry);
    expect([...container.querySelectorAll("a")].some((a) => a.textContent === entry)).toBe(false);
  }
});

it("詳細内には戻るボタンを重複表示せず、ヘッダーに任せる", async () => {
  apiFetch.mockResolvedValue(coach);
  await act(async () => root.render(<CoachDetailPage />));
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === "戻る")).toBe(false);
  expect(container.textContent).not.toContain("競技年数");
});

it("非公開または存在しないコーチの詳細には案内を出す", async () => {
  apiFetch.mockRejectedValue(new ApiRequestError(404, { error: { code: "coach_not_public", message: "非公開" } }));
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).toContain("このコーチは現在公開されていません");
});

it("選手側のコーチ詳細に平均評価は出ない（#628 の表示は廃止）", async () => {
  apiFetch.mockResolvedValue({ ...coach, rating_count: 3, rating_average: 4.5 });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).not.toContain("★4.5");
  expect(container.textContent).not.toContain("★");
  expect(container.textContent).toContain("テストコーチ");
});

it("選手側のコーチ詳細は英語でも平均評価は出ない（#628 の表示は廃止）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockResolvedValue({ ...coach, rating_count: 3, rating_average: 4.5 });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).not.toContain("★");
  expect(container.textContent).toContain("テストコーチ");
});

it("評価0件のときのコーチ詳細に「評価はまだありません」は出ない", async () => {
  apiFetch.mockResolvedValue(coach);
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).not.toContain("評価はまだありません");
  expect(container.textContent).toContain("テストコーチ");
});

it("公開プランのないコーチのカードには『プラン未公開』も空の価格行も出さない（一覧からは外さない）", async () => {
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachList />));
  const card = container.querySelector('li[id="coach-coach-1"]')!;
  expect(card.querySelectorAll("p")).toHaveLength(3);
  expect(container.textContent).not.toContain("プラン未公開");
});

it("公開プランのないコーチのカードを英語でも『No published plans』と出さない", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachList />));
  expect(container.textContent).not.toContain("No published plans");
  expect(container.textContent).not.toContain("プラン未公開");
});

it("公開中の継続プランがあるコーチのカードには価格を従来どおり出す", async () => {
  const supportPlan = { id: "plan-s", title: "1ヶ月プラン", kind: "single", description: "説明", duration_days: 30,
    price_yen: 5000, capacity: 3, published: true, service_type: "competition_support",
    video_reviews: 0, menu_updates: null, reply_formats: [], start_on_first_video: false };
  const varied = { ...coach, id: "coach-2", nickname: "幅ありコーチ", plans: [supportPlan, { ...supportPlan, id: "plan-s2", price_yen: 12000 }] };
  apiFetch.mockResolvedValue({ coaches: [{ ...coach, plans: [supportPlan] }, varied] });
  await act(async () => root.render(<CoachList />));
  const cards = [...container.querySelectorAll("li")];
  expect(cards[0].querySelectorAll("p")).toHaveLength(4);
  expect(cards[0].textContent).toContain("5,000円");
  expect(cards[0].textContent).not.toContain("プラン未公開");
  expect(cards[1].textContent).toContain("〜");
});

it("フォーム添削の一覧にも種目のチップが1列で並び、初期は「すべて」。選んだ種目のコーチだけを API の順のまま出す", async () => {
  apiFetch.mockResolvedValue({ coaches: [
    { ...coach, id: "coach-2", nickname: "跳躍", events: ["long-jump"] },
    { ...coach, id: "coach-1", events: ["100m", "200m", "400m"] },
    { ...coach, id: "coach-3", nickname: "200", events: ["200m"] },
  ] });
  await act(async () => root.render(<CoachList />));
  const group = container.querySelector('[role="radiogroup"][aria-label="種目で絞り込む"]')!;
  expect(group.className).toContain("flex-nowrap");
  expect(group.querySelector('[aria-checked="true"]')?.textContent).toBe("すべて");
  const hrefs = () => [...container.querySelectorAll("li a")].map((a) => a.getAttribute("href"));
  expect(hrefs()).toEqual(["/coaches/coach-2", "/coaches/coach-1", "/coaches/coach-3"]);
  const pick = (label: string) => act(async () => { ([...group.querySelectorAll("button")].find((b) => b.textContent?.replace("✓", "") === label) as HTMLButtonElement).click(); });
  await pick("200m");
  expect(hrefs()).toEqual(["/coaches/coach-1", "/coaches/coach-3"]);
  await pick("100m");
  expect(hrefs()).toEqual(["/coaches/coach-1"]);
  expect(container.textContent).not.toMatch(/★|ランキング|人気/);
});

it("フォーム添削の一覧で該当0人の種目は空状態と「すべて」へ戻る導線を出す", async () => {
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachList />));
  const group = container.querySelector('[role="radiogroup"][aria-label="種目で絞り込む"]')!;
  await act(async () => { ([...group.querySelectorAll("button")].find((b) => b.textContent === "やり投") as HTMLButtonElement).click(); });
  expect(container.querySelectorAll("li")).toHaveLength(0);
  expect(container.textContent).toContain("この種目のコーチはまだいません");
  await act(async () => { ([...container.querySelectorAll("button")].find((b) => b.textContent === "すべての種目のコーチを見る") as HTMLButtonElement).click(); });
  expect([...container.querySelectorAll("li a")].map((a) => a.getAttribute("href"))).toEqual(["/coaches/coach-1"]);
});

it("古い一覧データでは、詳細を取得する間に中央の読み込み表示を出す", async () => {
  apiFetch.mockReturnValue(new Promise(() => {}));
  rememberCoachDetailPreview(coach, Date.now() - 60_000);

  await act(async () => root.render(<CoachDetailPage />));
  expect(container.querySelector('[role="status"]')?.classList.contains("page-loading-screen")).toBe(true);
  expect(container.querySelector("h1")).toBeNull();
});

it("カードで選んだコーチをすぐに表示し、単体 API の応答に差し替える", async () => {
  apiFetch.mockResolvedValueOnce({ coaches: [coach] });
  let finish!: (value: PublicCoach) => void;
  apiFetch.mockReturnValueOnce(new Promise<PublicCoach>((resolve) => { finish = resolve; }));
  await act(async () => root.render(<CoachList />));
  await act(async () => container.querySelector("li a")!.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true })));

  await act(async () => root.render(<CoachDetailPage />));
  expect(container.querySelector("h1")?.textContent).toBe(coach.nickname);
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  expect(apiFetch.mock.calls).toEqual([["/v1/coaches"], ["/v1/coaches/coach-1"]]);

  await act(async () => finish({ ...coach, nickname: "更新後のコーチ" }));
  expect(container.querySelector("h1")?.textContent).toBe("更新後のコーチ");
});

it("一覧の公開ゲートで取得に失敗すると、プレビューを取り下げて読み込みエラーを出す", async () => {
  rememberCoachDetailPreview(coach, Date.now());
  apiFetch.mockRejectedValue(new ApiRequestError(404, { error: { code: "not_found", message: "見つかりませんでした" } }));
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.querySelector('[role="alert"]')?.textContent).toBe("コーチの詳細を読み込めませんでした");
  expect(container.querySelector("h1")).toBeNull();
});
