import { describe, expect, it } from "vitest";
import { practiceAttendanceLine, practiceScoreLabel } from "./practice-reputation";

describe("practiceAttendanceLine", () => {
  it("実績がない間は参加率を出さない", () => {
    expect(practiceAttendanceLine(undefined)).toBe("参加実績はまだありません");
    expect(practiceAttendanceLine({ total: 0, no_show: 0 })).toBe("参加実績はまだありません");
  });

  it("ドタキャンがなければ 100% にする", () => {
    expect(practiceAttendanceLine({ total: 4, no_show: 0 })).toBe("参加率 100%（4回中 ドタキャン0回）");
  });

  it("ドタキャンの分だけ参加率を下げる", () => {
    expect(practiceAttendanceLine({ total: 8, no_show: 1 })).toBe("参加率 88%（8回中 ドタキャン1回）");
  });
});

describe("practiceScoreLabel", () => {
  it("no_show をドタキャンと表示する", () => {
    expect(practiceScoreLabel("no_show")).toBe("ドタキャン");
  });
});
