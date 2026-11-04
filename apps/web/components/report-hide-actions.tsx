"use client";

import Link from "next/link";
import { useState } from "react";
import { hideConfirmOptions, useConfirm, type ConfirmOptions } from "@/components/confirm-dialog";
import { ApiRequestError, apiErrorText, apiFetch } from "@/lib/api";
import type { AdminReport } from "@/lib/types";

type HideTarget = { confirm: ConfirmOptions; path: (id: string) => string };

const hideLabel = "非表示にする";

// 通報の対象ごとの、非表示の確認文と API。
export const hideTargets: Partial<Record<AdminReport["target_type"], HideTarget>> = {
  post: { confirm: hideConfirmOptions("この投稿"), path: (id) => `/v1/admin/posts/${id}/hide` },
  comment: { confirm: hideConfirmOptions("このコメント"), path: (id) => `/v1/admin/comments/${id}/hide` },
  injury_note: { confirm: { title: "この一言を公開ページから外しますか？", confirmLabel: hideLabel }, path: (id) => `/v1/admin/injury-notes/${id}/hide` },
  review_response: { confirm: { title: "この返答を利用者から見えなくしますか？", confirmLabel: hideLabel }, path: (id) => `/v1/admin/coaching/responses/${id}/hide` },
  practice_rating: { confirm: { title: "このひとことを練習の投稿から外しますか？", confirmLabel: hideLabel }, path: (id) => `/v1/admin/practice-ratings/${id}/hide` },
};

export function ReportHideActions({ report, token }: { report: AdminReport; token: string | null }) {
  const [hidden, setHidden] = useState(report.target_hidden);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const confirmDialog = useConfirm();
  const target = hideTargets[report.target_type];

  async function hide() {
    if (!target || !(await confirmDialog(target.confirm))) return;
    setPending(true);
    try {
      await apiFetch(target.path(report.target_id), { method: "POST", token });
      setHidden(true);
      setError("");
    } catch (err) {
      // 通報の一覧は読み込んだ時点のものなので、その後に消えた対象でも押せてしまう。
      setError(err instanceof ApiRequestError && err.status === 404 ? "すでに非表示か、削除されています" : apiErrorText(err, "非表示にできませんでした"));
    } finally {
      setPending(false);
    }
  }

  // 非表示にした投稿は開けないので、投稿へのリンクは出さない。コメントの投稿は開ける。
  const showPostLink = report.target_type === "comment" || (report.target_type === "post" && !hidden);

  return (
    <div className="flex flex-wrap items-center gap-3 text-sm">
      {report.target_type === "injury_note" ? (
        <Link href={`/injury/${report.injury_part}`} className="text-blue-300 underline">
          公開ページを見る
        </Link>
      ) : null}
      {showPostLink ? (
        <Link href={`/posts/${report.post_id}`} className="text-blue-300 underline">
          投稿を見る
        </Link>
      ) : null}
      {hidden ? (
        <span className="text-slate-300">非表示済み</span>
      ) : (
        <button type="button" disabled={pending} onClick={() => void hide()} className="text-red-300 underline disabled:opacity-50">
          {hideLabel}
        </button>
      )}
      {error ? <span className="text-red-400">{error}</span> : null}
    </div>
  );
}
