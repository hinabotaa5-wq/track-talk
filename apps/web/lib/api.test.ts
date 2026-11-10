import { afterEach, describe, expect, it, vi } from "vitest";
import { apiFetch, ApiRequestError, apiErrorText } from "./api";
import { installEnglish, setActiveLocale } from "./i18n";
import { en } from "./i18n-en";

afterEach(() => vi.unstubAllGlobals());

describe("successful operation tracking", () => {
  it.each([201, 204])("counts a successful response (%s) once", async (status) => {
    const track = vi.fn();
    vi.stubGlobal("window", { location: { hostname: "track-talk.netlify.app" }, umami: { track } });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(status === 204 ? null : '{"id":"123"}', { status })));
    await apiFetch("/v1/posts", { method: "POST", body: '{"body":"private"}', token: "secret" });
    expect(track).toHaveBeenCalledExactlyOnceWith("post_created");
  });

  it("does not count failed HTTP responses or network failures", async () => {
    const track = vi.fn();
    vi.stubGlobal("window", { location: { hostname: "track-talk.netlify.app" }, umami: { track } });
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

describe("API のエラー文の表示言語（本体 #530）", () => {
  afterEach(() => setActiveLocale("ja"));

  it("日本語の表示では API の文をそのまま出す", () => {
    const err = new ApiRequestError(422, { error: { code: "validation_error", message: "入力内容を確認してください", fields: { body: "本文は1〜200文字で入力してください" } } });
    expect(err.message).toBe("入力内容を確認してください");
    expect(apiErrorText(err, "")).toBe("本文は1〜200文字で入力してください");
  });

  it("英語の表示では辞書の英語にし、辞書に無い文は日本語を混ぜずに一般的な英語にする", () => {
    installEnglish(en);
    setActiveLocale("en");
    expect(new ApiRequestError(404, { error: { code: "not_found", message: "見つかりませんでした" } }).message).toBe("Not found");
    const unknown = new ApiRequestError(422, { error: { code: "x", message: "辞書に無いエラーの文です", fields: { a: "辞書に無い項目の指摘です" } } });
    expect(unknown.message).toBe("This can't be done. Please check your input and try again.");
    expect(apiErrorText(unknown, "")).toBe("This can't be done. Please check your input and try again.");
  });
});
