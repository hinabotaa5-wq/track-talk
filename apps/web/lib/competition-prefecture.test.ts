import { afterEach, describe, expect, it, vi } from "vitest";
import {
  legacyCompetitionsTarget,
  regionalPrefecture,
  regionalRedirectTarget,
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

  it("sends a bare regional visit back to the saved prefecture", () => {
    expect(regionalRedirectTarget(undefined, "13", undefined)).toBe("/competitions/regional?prefecture=13");
    expect(regionalRedirectTarget(undefined, "13", "4")).toBe("/competitions/regional?month=4&prefecture=13");
    expect(regionalRedirectTarget("27", "13", undefined)).toBe("");
    expect(regionalRedirectTarget(undefined, INTERNATIONAL, undefined)).toBe("");
    expect(regionalRedirectTarget(undefined, undefined, undefined)).toBe("");
  });

  it("moves old main tab urls to their new place", () => {
    expect(legacyCompetitionsTarget(undefined, "4")).toBe("");
    expect(legacyCompetitionsTarget("00", "4")).toBe("/competitions?view=international");
    expect(legacyCompetitionsTarget("13", "4")).toBe("/competitions/regional?month=4&prefecture=13");
    expect(legacyCompetitionsTarget("99", undefined)).toBe("");
  });

  it("does not treat 国際 as a prefecture", () => {
    expect(regionalPrefecture("13")).toBe("13");
    expect(regionalPrefecture(INTERNATIONAL)).toBe("");
    expect(regionalPrefecture(undefined)).toBe("");
  });
});
