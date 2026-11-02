import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { InjuryPreventionCardView, preventionLines, type InjuryPrevention } from "./injury-prevention-card";

const tally = (items: [string, string, number][], people = 6) => ({
  people,
  items: items.map(([code, label, count]) => ({ code, label, people: count })),
});

const data: InjuryPrevention = {
  event: "400m",
  event_label: "400m",
  min_people: 5,
  body_parts: tally([["hamstring-strain", "ハムストリングの肉離れ", 4], ["sprain", "足首の捻挫", 2]]),
  pre_changes: tally([["more-volume", "練習量を増やした", 3]]),
  hindsight: tally([["adjust-volume", "練習量や強度を調整する", 5]]),
};

describe("InjuryPreventionCardView", () => {
  it("shows the most common item of each question for the event", () => {
    const html = renderToStaticMarkup(<InjuryPreventionCardView data={data} />);
    expect(html).toContain("400mの人のケガの傾向");
    expect(html).toContain("ハムストリングの肉離れ");
    expect(html).not.toContain("足首の捻挫");
    expect(html).toContain("練習量を増やした");
    expect(html).toContain("練習量や強度を調整する");
    expect(html).toContain('href="/injury/data"');
  });

  it("skips questions that have not reached the minimum and hides the card when none have", () => {
    const partial = { ...data, pre_changes: tally([], 3), hindsight: tally([], 2) };
    expect(preventionLines(partial).map((line) => line.label)).toEqual(["多いケガ"]);

    const empty = { ...partial, body_parts: tally([], 4) };
    expect(renderToStaticMarkup(<InjuryPreventionCardView data={empty} />)).toBe("");
  });
});
