import { expect, it } from "vitest";
import { minorDeclarationBody, purchaseNoticeShown } from "./minor-purchase";

it("購入前チェックは、API が成人と判定した人（false）にだけ出さない。値がなければ安全側で出す（本体 #1045）", () => {
  expect(purchaseNoticeShown({ purchase_notice_required: false })).toBe(false);
  expect(purchaseNoticeShown({ purchase_notice_required: true })).toBe(true);
  expect(purchaseNoticeShown({})).toBe(true);
  expect(purchaseNoticeShown(null)).toBe(true);
});

it("18歳以上には何も送らない", () => {
  expect(minorDeclarationBody(false, true, { shownAt: "2026-11-05T00:00:00.000Z" })).toEqual({});
});

it("18歳未満には、チェックと、表示・チェックの時刻を送る", () => {
  expect(minorDeclarationBody(true, true, { shownAt: "2026-11-05T00:00:00.000Z", checkedAt: "2026-11-05T00:00:30.000Z" })).toEqual({
    guardian_permission: true,
    guardian_permission_shown_at: "2026-11-05T00:00:00.000Z",
    guardian_permission_checked_at: "2026-11-05T00:00:30.000Z",
  });
});

it("チェックがないときは、チェックの時刻を送らない", () => {
  expect(minorDeclarationBody(true, false, { shownAt: "2026-11-05T00:00:00.000Z", checkedAt: "2026-11-05T00:00:30.000Z" })).toEqual({
    guardian_permission: false,
    guardian_permission_shown_at: "2026-11-05T00:00:00.000Z",
  });
});
