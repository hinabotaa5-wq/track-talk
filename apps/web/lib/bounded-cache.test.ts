import { expect, it, vi } from "vitest";
import { BoundedCache } from "./bounded-cache";

it("最近使われた値を残し、上限と期限を守る", () => {
  let now = 0;
  vi.spyOn(Date, "now").mockImplementation(() => now);
  try {
    const cache = new BoundedCache<number>(2, 1000);
    cache.set("a", 1); cache.set("b", 2);
    expect(cache.get("a")).toBe(1);
    cache.set("c", 3);
    expect(cache.get("b")).toBeUndefined();
    expect([...cache.keys()]).toHaveLength(2);
    now = 1000;
    expect(cache.get("a")).toBeUndefined();
    expect(cache.get("c")).toBeUndefined();
  } finally { vi.restoreAllMocks(); }
});
