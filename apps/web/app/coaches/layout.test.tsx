import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import NewReviewRequestLayout from "../coaching/new/layout";
import CoachesLayout from "./layout";
import CoachingPage from "../coaching/page";

// コーチ一覧の公開は、コーチングの開放（NEXT_PUBLIC_COACHING_ENABLED）とは別のフラグで切り替える。
const flags = { coaching: false, list: false };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
  get coachingCoachListEnabled() { return flags.list; },
}));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/coach-apply-link", () => ({ CoachApplyLink: () => <a href="/coach/apply">コーチに応募する</a> }));

beforeEach(() => {
  flags.coaching = false;
  flags.list = false;
});

const page = <p>中身</p>;
const preparing = "ただいま準備中です";
const startNotice = "11/14(土)に開始します！";

afterEach(() => vi.unstubAllEnvs());

// 開始の告知は日付で自動に出さず、環境変数で文言ごと差し替える。既定（未設定）は出さない（本体 #569）。
it("告知の環境変数が空の間は、どの画面にも開始の告知を出さない", () => {
  vi.stubEnv("NEXT_PUBLIC_VIDEO_REVIEW_NOTICE", "");
  for (const html of [renderToStaticMarkup(<CoachingPage />), renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>), renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>)]) {
    expect(html).not.toContain(startNotice);
    expect(html).not.toContain("開始予定");
  }
});

it("告知の環境変数に文言を入れると、その文言を3画面に出す", () => {
  vi.stubEnv("NEXT_PUBLIC_VIDEO_REVIEW_NOTICE", startNotice);
  expect(renderToStaticMarkup(<CoachingPage />)).toContain(startNotice);
  expect(renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>)).toContain(startNotice);
  expect(renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>)).toContain(startNotice);
});

it("告知を出していても、コーチングが開いていれば申込みはそのまま開く", () => {
  vi.stubEnv("NEXT_PUBLIC_VIDEO_REVIEW_NOTICE", startNotice);
  flags.coaching = true;
  expect(renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>)).toBe("<p>中身</p>");
});

it.each([false, true])("コーチングを閉じている間は、一覧のフラグ（%s）によらず一覧も申込みも準備中（現行どおり）", (list) => {
  flags.list = list;
  for (const html of [renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>), renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>)]) {
    expect(html).toContain(preparing);
    expect(html).not.toContain("中身");
  }
});

it("コーチングを開けて一覧を閉じている間は、一覧だけ同じ準備中にし、申込みは開く", () => {
  flags.coaching = true;
  const closed = renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>);
  flags.coaching = false;
  // 文言はコーチングを閉じたときの準備中と同じ。
  expect(closed).toBe(renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>));
  expect(closed).toContain(preparing);
  expect(closed).not.toContain("中身");
  flags.coaching = true;
  expect(renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>)).toBe("<p>中身</p>");
});

it("コーチングと一覧の両方を開けると、一覧を表示する", () => {
  flags.coaching = true;
  flags.list = true;
  expect(renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>)).toBe("<p>中身</p>");
  expect(renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>)).toBe("<p>中身</p>");
});
