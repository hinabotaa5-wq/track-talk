import { afterEach, describe, expect, it, vi } from "vitest";
import { apiFetch, ApiRequestError, apiErrorText } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("successful operation tracking", () => {
  it.each([201, 204])("counts a successful response (%s) once", async (status) => {
    const track = vi.fn();
    vi.stubGlobal("window", { location: { hostname: "tracktalk.jp" }, umami: { track } });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(status === 204 ? null : '{"id":"123"}', { status })));
    await apiFetch("/v1/posts", { method: "POST", body: '{"body":"private"}', token: "secret" });
    expect(track).toHaveBeenCalledExactlyOnceWith("post_created");
  });

  it("does not count failed HTTP responses or network failures", async () => {
    const track = vi.fn();
    vi.stubGlobal("window", { location: { hostname: "tracktalk.jp" }, umami: { track } });
    const fetch = vi.fn().mockResolvedValueOnce(new Response("{}", { status: 422 })).mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetch);
    await expect(apiFetch("/v1/posts", { method: "POST" })).rejects.toBeInstanceOf(ApiRequestError);
    await expect(apiFetch("/v1/posts", { method: "POST" })).rejects.toBeInstanceOf(ApiRequestError);
    expect(track).not.toHaveBeenCalled();
  });
});

describe("cache", () => {
  it("ふだんは取り置かず、revalidate を渡したときだけ取り置きを使う", async () => {
    const fetch = vi.fn().mockImplementation(async () => new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetch);
    await apiFetch("/v1/domestic-majors");
    await apiFetch("/v1/domestic-majors", { next: { revalidate: 600 } });
    expect(fetch.mock.calls[0][1].cache).toBe("no-store");
    expect(fetch.mock.calls[1][1].cache).toBeUndefined();
    expect(fetch.mock.calls[1][1].next).toEqual({ revalidate: 600 });
  });
});

describe("apiErrorText", () => {
  it("項目ごとの指摘があれば最初の1つを出す", () => {
    const err = new ApiRequestError(422, {
      error: { code: "validation", message: "入力を確かめてください", fields: { title: "タイトルを入れてね", body: "本文を入れてね" } },
    });
    expect(apiErrorText(err, "保存できませんでした")).toBe("タイトルを入れてね");
  });

  it("項目の指摘がなければ API のメッセージを出す", () => {
    expect(apiErrorText(new ApiRequestError(403, { error: { code: "forbidden", message: "できません" } }), "x")).toBe("できません");
    expect(apiErrorText(new ApiRequestError(422, { error: { code: "validation", message: "だめ", fields: {} } }), "x")).toBe("だめ");
  });

  it("API 以外のエラーは fallback を出す", () => {
    expect(apiErrorText(new Error("network"), "保存できませんでした")).toBe("保存できませんでした");
    expect(apiErrorText("oops", "保存できませんでした")).toBe("保存できませんでした");
  });
});
