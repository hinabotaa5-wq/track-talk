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
const startNotice = "動画添削は 2026年11月14日（土）に開始予定です。";

afterEach(() => vi.useRealTimers());

it("11月14日午前0時（日本時間）の直前は3画面で動画添削の開始予定を案内する", () => {
  vi.setSystemTime(new Date("2026-11-13T23:59:59.999+09:00"));
  expect(renderToStaticMarkup(<CoachingPage />)).toContain(startNotice);
  expect(renderToStaticMarkup(<NewReviewRequestLayout>{page}</NewReviewRequestLayout>)).toContain(startNotice);
  expect(renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>)).toContain(startNotice);
});

it("11月14日午前0時（日本時間）になると一覧の準備中表示から開始予定を外す", () => {
  vi.setSystemTime(new Date("2026-11-14T00:00:00+09:00"));
  const html = renderToStaticMarkup(<CoachesLayout>{page}</CoachesLayout>);
  expect(html).toContain(preparing);
  expect(html).not.toContain(startNotice);
});

it("開始予定日前でもコーチングが開いていればトップと申込みに開始予定を出さない", () => {
  vi.setSystemTime(new Date("2026-11-13T12:00:00+09:00"));
  flags.coaching = true;
  expect(renderToStaticMarkup(<CoachingPage />)).not.toContain(startNotice);
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
