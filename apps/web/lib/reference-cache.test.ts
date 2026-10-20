import { beforeEach, describe, expect, it, vi } from "vitest";

let resolvers: Array<(v: unknown) => void> = [];
vi.mock("./api", () => ({
  apiFetch: () => new Promise((r) => resolvers.push(r)),
}));

import { clearCompetitions, loadCompetitions, readCompetitions } from "./competition-list";
import { clearDomesticMajors, loadDomesticMajors, peekDomesticMajors } from "./domestic-majors";
import { clearMajorHosts, loadMajorHosts, peekMajorHosts } from "./major-hosts";

beforeEach(() => {
  resolvers = [];
});

describe("reference data caches", () => {
  it("competition list does not write back a request that started before clearing", async () => {
    const pending = loadCompetitions("13", 0);
    clearCompetitions();
    resolvers[0]({ competitions: [{ id: "old" }] });
    await pending;
    expect(readCompetitions("13", 0)).toBeUndefined();
  });

  it("major hosts do not write back a request that started before clearing", async () => {
    const pending = loadMajorHosts();
    clearMajorHosts();
    resolvers[0]({ hosts: [{ kind: "worlds" }] });
    await pending;
    expect(peekMajorHosts()).toBeNull();
  });

  it("domestic majors do not write back a request that started before clearing", async () => {
    const pending = loadDomesticMajors();
    clearDomesticMajors();
    resolvers[0]({ majors: [{ slug: "old" }] });
    await pending;
    expect(peekDomesticMajors()).toBeNull();
  });

  it("caches a request that was not cleared", async () => {
    const pending = loadMajorHosts();
    resolvers[0]({ hosts: [{ kind: "worlds" }] });
    await pending;
    expect(peekMajorHosts()).toHaveLength(1);
    clearMajorHosts();
  });
});
