/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import type { PublicCoach } from "@/lib/types";
import { CoachList } from "./coach-list";
import CoachDetailPage from "./[id]/page";

const apiFetch = vi.fn();
const routerBack = vi.fn();
const routerPush = vi.fn();
vi.mock("@/lib/api", () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "coach-1" }), useRouter: () => ({ back: routerBack, push: routerPush }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
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
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachDetailPage />));
  expect(apiFetch).toHaveBeenCalledWith("/v1/coaches");
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
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachDetailPage />));
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === "戻る")).toBe(false);
  expect(container.textContent).not.toContain("競技年数");
});

it("非公開または存在しないコーチの詳細には案内を出す", async () => {
  apiFetch.mockResolvedValue({ coaches: [] });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).toContain("このコーチは現在公開されていません");
});

it("選手側のコーチ詳細の平均評価は件数を出さない（#628）", async () => {
  apiFetch.mockResolvedValue({ coaches: [{ ...coach, rating_count: 3, rating_average: 4.5 }] });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).toContain("★4.5");
  expect(container.textContent).not.toContain("3件");
  expect(container.textContent).not.toContain("(3)");
});

it("選手側のコーチ詳細は英語でも平均評価の件数を出さない（#628）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockResolvedValue({ coaches: [{ ...coach, rating_count: 3, rating_average: 4.5 }] });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).toContain("★4.5");
  expect(container.textContent).not.toContain("(3)");
  expect(container.textContent).not.toContain("3件");
});

it("評価0件のときのコーチ詳細の表示は変えない（#628）", async () => {
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachDetailPage />));
  expect(container.textContent).toContain("評価はまだありません");
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

it("公開中のサポートプランがあるコーチのカードには価格を従来どおり出す", async () => {
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

it("動画添削の一覧にも種目のチップが1列で並び、初期は「すべて」。選んだ種目のコーチだけを API の順のまま出す", async () => {
  apiFetch.mockResolvedValue({ coaches: [
    { ...coach, id: "coach-2", nickname: "跳躍", events: ["long-jump"] },
    { ...coach, id: "coach-1", events: ["100m", "200m", "400m"] },
    { ...coach, id: "coach-3", nickname: "200", events: ["200m"] },
  ] });
  await act(async () => root.render(<CoachList />));
  const group = container.querySelector('[role="radiogroup"][aria-label="種目で絞り込む"]')!;
  expect(group.className).toContain("flex-nowrap");
  expect(group.querySelector('[aria-checked="true"]')?.textContent).toBe("✓すべて");
  const hrefs = () => [...container.querySelectorAll("li a")].map((a) => a.getAttribute("href"));
  expect(hrefs()).toEqual(["/coaches/coach-2", "/coaches/coach-1", "/coaches/coach-3"]);
  const pick = (label: string) => act(async () => { ([...group.querySelectorAll("button")].find((b) => b.textContent?.replace("✓", "") === label) as HTMLButtonElement).click(); });
  await pick("200m");
  expect(hrefs()).toEqual(["/coaches/coach-1", "/coaches/coach-3"]);
  await pick("100m");
  expect(hrefs()).toEqual(["/coaches/coach-1"]);
  expect(container.textContent).not.toMatch(/★|ランキング|人気/);
});

it("動画添削の一覧で該当0人の種目は空状態と「すべて」へ戻る導線を出す", async () => {
  apiFetch.mockResolvedValue({ coaches: [coach] });
  await act(async () => root.render(<CoachList />));
  const group = container.querySelector('[role="radiogroup"][aria-label="種目で絞り込む"]')!;
  await act(async () => { ([...group.querySelectorAll("button")].find((b) => b.textContent === "やり投") as HTMLButtonElement).click(); });
  expect(container.querySelectorAll("li")).toHaveLength(0);
  expect(container.textContent).toContain("この種目のコーチはまだいません");
  await act(async () => { ([...container.querySelectorAll("button")].find((b) => b.textContent === "すべての種目のコーチを見る") as HTMLButtonElement).click(); });
  expect([...container.querySelectorAll("li a")].map((a) => a.getAttribute("href"))).toEqual(["/coaches/coach-1"]);
});
