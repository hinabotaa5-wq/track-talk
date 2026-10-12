import { afterEach, describe, expect, it, vi } from "vitest";
import {
  competitionsPath,
  competitionsRedirectTarget,
  readSavedCompetitionPrefecture,
  saveCompetitionPrefecture,
} from "./competition-prefecture";
import { INTERNATIONAL } from "./prefectures";

const store = new Map<string, string>();
let cookie = "";

afterEach(() => {
  store.clear();
  cookie = "";
  vi.unstubAllGlobals();
});

function stubStorage() {
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
    },
    dispatchEvent: () => true,
  });
  vi.stubGlobal("document", {
    get cookie() {
      return cookie;
    },
    set cookie(value: string) {
      cookie = value;
    },
  });
  vi.stubGlobal("Event", class Event {
    constructor(public type: string) {}
  });
}

describe("competition prefecture memory", () => {
  it("remembers a selected prefecture and ignores an invalid code", () => {
    stubStorage();
    saveCompetitionPrefecture("13");
    expect(readSavedCompetitionPrefecture()).toBe("13");
    expect(cookie).toContain("tt_competition_prefecture=13");
    saveCompetitionPrefecture("99");
    expect(readSavedCompetitionPrefecture()).toBe("13");
    saveCompetitionPrefecture(INTERNATIONAL);
    expect(readSavedCompetitionPrefecture()).toBe(INTERNATIONAL);
  });

  it("builds the competitions tab url from the saved region", () => {
    expect(competitionsPath("")).toBe("/competitions");
    expect(competitionsPath("13")).toBe("/competitions?prefecture=13");
    expect(competitionsPath(INTERNATIONAL)).toBe("/competitions?prefecture=00");
  });

  it("sends a bare competitions visit back to the saved region", () => {
    expect(competitionsRedirectTarget(undefined, "13", undefined)).toBe("/competitions?prefecture=13");
    expect(competitionsRedirectTarget(undefined, "13", "4")).toBe("/competitions?month=4&prefecture=13");
    expect(competitionsRedirectTarget("27", "13", undefined)).toBe("");
    expect(competitionsRedirectTarget(undefined, INTERNATIONAL, undefined)).toBe("");
    expect(competitionsRedirectTarget(undefined, undefined, undefined)).toBe("");
  });
});
