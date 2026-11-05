import { expect, it } from "vitest";
import { minorDeclarationBody } from "./minor-purchase";

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
