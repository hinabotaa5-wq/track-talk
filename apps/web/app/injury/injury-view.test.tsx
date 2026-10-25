import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { InjuryGroupSummary } from "@/lib/injury";
import { InjuryGroupPicker } from "./injury-view";

const groups: InjuryGroupSummary[] = [
  { body_part: "shin-splints", body_part_label: "シンスプリント", members: 4 },
  { body_part: "jumpers-knee", body_part_label: "ジャンパー膝", members: 2 },
  {
    body_part: "hamstring-strain",
    body_part_label: "ハムストリングの肉離れ",
    members: 6,
    me: { stage: "rehab", stage_label: "リハビリ中", injured_on: "2026-10-01", injured_on_exact: true, pre_changes: [] },
  },
];

describe("InjuryGroupPicker", () => {
  it("opens on the region of the group I joined and lists only its injuries", () => {
    const html = renderToStaticMarkup(<InjuryGroupPicker groups={groups} />);
    expect(html).toMatch(/aria-pressed="true"[^>]*>太もも</);
    expect(html).toContain("ハムストリングの肉離れ");
    expect(html).toContain("参加中 · リハビリ中");
    expect(html).toContain('href="/injury/hamstring-strain"');
    expect(html).not.toContain("シンスプリント");
  });

  it("shows how many people are in each region on the body", () => {
    const html = renderToStaticMarkup(<InjuryGroupPicker groups={groups.filter((group) => !group.me)} />);
    expect(html).toMatch(/aria-pressed="true"[^>]*>ひざ・すね</);
    expect(html).toMatch(/>6<\/text>/);
    expect(html).toContain("グループに入る");
  });
});
