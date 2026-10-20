import { describe, expect, it } from "vitest";
import { fetchAllPages } from "./paging";

function pager(total: number, per = 50) {
  const calls: number[] = [];
  const fetchPage = async (page: number) => {
    calls.push(page);
    const start = (page - 1) * per;
    const items = Array.from({ length: Math.max(0, Math.min(per, total - start)) }, (_, i) => start + i);
    return { items, total };
  };
  return { calls, fetchPage };
}

describe("fetchAllPages", () => {
  it("collects every page until the total is reached", async () => {
    const { calls, fetchPage } = pager(120);
    const { items, total } = await fetchAllPages(fetchPage);
    expect(items).toHaveLength(120);
    expect(total).toBe(120);
    expect(calls).toEqual([1, 2, 3]);
  });

  it("stops after one page when everything fits", async () => {
    const { calls, fetchPage } = pager(10);
    expect((await fetchAllPages(fetchPage)).items).toHaveLength(10);
    expect(calls).toEqual([1]);
  });

  it("stops on an empty page even if total says more", async () => {
    const { calls, fetchPage } = pager(0);
    const out = await fetchAllPages(async (p) => ({ ...(await fetchPage(p)), total: 500 }));
    expect(out.items).toEqual([]);
    expect(calls).toEqual([1]);
  });

  it("stops at maxPages", async () => {
    const { calls, fetchPage } = pager(10_000);
    const out = await fetchAllPages(fetchPage, 3);
    expect(out.items).toHaveLength(150);
    expect(calls).toEqual([1, 2, 3]);
  });
});
