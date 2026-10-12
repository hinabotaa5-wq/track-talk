import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { UploadProgress } from "./upload-progress";

function markup(props: Parameters<typeof UploadProgress>[0]) {
  return renderToStaticMarkup(createElement(UploadProgress, props));
}

describe("UploadProgress", () => {
  it("shows how much of the file has been sent", () => {
    const html = markup({
      noun: "動画",
      phase: "upload",
      progress: { loaded: 20 * 1024 * 1024, total: 40 * 1024 * 1024, percent: 50 },
      fileName: "race.mp4",
      fileSize: 40 * 1024 * 1024,
    });
    expect(html).toContain("動画を送っています");
    expect(html).toContain("50%");
    expect(html).toContain("width:50%");
    expect(html).toContain("20.0 MB / 40.0 MB");
    expect(html).toContain("race.mp4");
    expect(html).toContain("このあと投稿ページに移動します");
  });

  it("stays under 100 until the whole file is sent", () => {
    const html = markup({
      noun: "写真",
      phase: "upload",
      progress: { loaded: 996, total: 1000, percent: 99.6 },
    });
    expect(html).toContain("写真を送っています");
    expect(html).toContain("99%");
    expect(html).not.toContain("取り込み中");
    expect(html).not.toContain("送り終わりました");
  });

  it("moves without a percentage once the file has left the browser", () => {
    const html = markup({
      noun: "動画",
      phase: "upload",
      progress: { loaded: 40 * 1024 * 1024, total: 40 * 1024 * 1024, percent: 100 },
      fileName: "race.mp4",
      fileSize: 40 * 1024 * 1024,
    });
    expect(html).toContain("取り込み中");
    expect(html).toContain("race.mp4");
    expect(html).toContain("40.0 MB");
    expect(html).toContain("upload-indeterminate");
    expect(html).toContain("このあと投稿ページに移動します");
    expect(html).not.toContain("100%");
    expect(html).not.toContain("aria-valuenow");
    expect(html).not.toContain("送り終わりました");
  });

  it("keeps moving while the post is saved", () => {
    const html = markup({
      noun: "動画",
      phase: "saving",
      progress: { loaded: 40 * 1024 * 1024, total: 40 * 1024 * 1024, percent: 100 },
    });
    expect(html).toContain("投稿を保存しています");
    expect(html).toContain("upload-indeterminate");
    expect(html).not.toContain("100%");
  });
});
