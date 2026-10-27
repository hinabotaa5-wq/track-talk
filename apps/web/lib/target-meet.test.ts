import { expect, it } from "vitest";
import { targetMeetCountdown } from "./target-meet";

it("初日の前はあと何日、初日は今日、会期中は開催中、最終日の翌日から終了", () => {
  const meet = { starts_on: "2026-11-20", ends_on: "2026-11-22" };
  expect(targetMeetCountdown(meet, "2026-10-27")).toEqual({ kind: "before", days: 24 });
  expect(targetMeetCountdown(meet, "2026-11-19")).toEqual({ kind: "before", days: 1 });
  expect(targetMeetCountdown(meet, "2026-11-20")).toEqual({ kind: "today" });
  expect(targetMeetCountdown(meet, "2026-11-22")).toEqual({ kind: "ongoing" });
  expect(targetMeetCountdown(meet, "2026-11-23")).toEqual({ kind: "over" });
  expect(targetMeetCountdown({ starts_on: "2026-11-20", ends_on: null }, "2026-11-21")).toEqual({ kind: "over" });
  // 年をまたいでも日数がずれない。
  expect(targetMeetCountdown({ starts_on: "2027-01-02", ends_on: null }, "2026-12-31")).toEqual({ kind: "before", days: 2 });
});
