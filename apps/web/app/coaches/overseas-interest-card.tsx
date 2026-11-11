"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { useAuth } from "@/components/auth-provider";
import { ApiRequestError, apiErrorText, apiFetch } from "@/lib/api";
import { overseasInterestEnabled } from "@/lib/features";
import { L } from "@/lib/i18n";
import type { OverseasInterestKind } from "@/lib/types";

const kinds: OverseasInterestKind[] = ["monthly", "single", "both"];

function kindLabel(kind: OverseasInterestKind): string {
  switch (kind) {
    case "monthly":
      return L("1ヶ月コーチング");
    case "single":
      return L("単発の動画添削");
    case "both":
      return L("両方");
  }
}

// 海外コーチの指導希望の集計カード（本体 #542）。コーチタブの独立カード。
// 一覧・サポートプランの入口とは混ぜず、タブの下に1枚だけ置く。登録数は出さない。
export function OverseasInterestCard() {
  const { token, loading } = useAuth();
  const [interest, setInterest] = useState<OverseasInterestKind | null | undefined>(undefined);
  const [loginNeeded, setLoginNeeded] = useState(false);
  const [selected, setSelected] = useState<OverseasInterestKind | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const data = await apiFetch<{ interest: { kind: OverseasInterestKind } | null }>("/v1/me/overseas-interest", { token });
      setInterest(data.interest?.kind ?? null);
      setSelected(data.interest?.kind ?? null);
      setLoginNeeded(false);
      setError("");
    } catch (err) {
      // 未ログイン（トークンなし・切れ）は登録できないので、ログイン導線を出す。
      if (err instanceof ApiRequestError && err.status === 401) {
        setLoginNeeded(true);
        return;
      }
      setError(apiErrorText(err, L("読み込みに失敗しました")));
    }
  }, [token]);

  useEffect(() => {
    if (!overseasInterestEnabled) return;
    if (loading) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [loading, load]);

  if (!overseasInterestEnabled) return null;
  if (loading) return <p className="text-sm text-slate-200">{L("読み込み中...")}</p>;

  const heading = L("海外コーチ、近日開始");
  const body = L(
    "海外在住のスプリントコーチから指導を受けられる機能を準備中です（2026年12月中旬開始予定）。受けてみたい方は、下のボタンで教えてください。開始の際にお知らせします。",
  );

  if (loginNeeded) {
    return (
      <section aria-label={heading} className="space-y-2 rounded-xl border border-blue-400/40 bg-blue-500/10 p-4">
        <h2 className="font-bold text-white">{heading}</h2>
        <p className="text-sm text-slate-200">{body}</p>
        <Link href="/login?next=/coaches" className="inline-block rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white">
          {L("ログインして登録する")}
        </Link>
      </section>
    );
  }

  if (interest === undefined) {
    return <p className="text-sm text-slate-200">{error || L("読み込み中...")}</p>;
  }

  async function submit(kind: OverseasInterestKind) {
    setSaving(true);
    setError("");
    try {
      const data = await apiFetch<{ interest: { kind: OverseasInterestKind } }>("/v1/me/overseas-interest", {
        method: "PUT",
        token,
        body: JSON.stringify({ kind }),
      });
      setInterest(data.interest.kind);
      setSelected(data.interest.kind);
    } catch (err) {
      setError(apiErrorText(err, L("登録できませんでした")));
    } finally {
      setSaving(false);
    }
  }

  async function cancel() {
    setSaving(true);
    setError("");
    try {
      await apiFetch<{ interest: null }>("/v1/me/overseas-interest", { method: "DELETE", token });
      setInterest(null);
      setSelected(null);
    } catch (err) {
      setError(apiErrorText(err, L("登録を取り消せませんでした")));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section aria-label={heading} className="space-y-3 rounded-xl border border-blue-400/40 bg-blue-500/10 p-4">
      <h2 className="font-bold text-white">{heading}</h2>
      <p className="text-sm text-slate-200">{body}</p>
      {interest !== null ? (
        <>
          <p className="text-sm font-semibold text-white">{L("登録済みです。開始時にお知らせします")}</p>
          <p className="text-sm text-slate-200">{L("いまの選択：{v0}", { v0: kindLabel(interest) })}</p>
        </>
      ) : null}
      <fieldset className="space-y-1">
        <legend className="text-sm font-semibold text-slate-100">{L("受けたい形を選んでください")}</legend>
        {kinds.map((kind) => (
          <label key={kind} className="flex items-center gap-2 text-sm text-slate-100">
            <input
              type="radio"
              name="overseas-interest-kind"
              checked={selected === kind}
              onChange={() => setSelected(kind)}
            />
            {kindLabel(kind)}
          </label>
        ))}
      </fieldset>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <div className="flex gap-2">
        <button
          type="button"
          disabled={saving || selected === null || selected === interest}
          onClick={() => { if (selected !== null) void submit(selected); }}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50"
        >
          {interest === null ? L("指導を受けたい") : L("変更する")}
        </button>
        {interest !== null ? (
          <button
            type="button"
            disabled={saving}
            onClick={() => void cancel()}
            className="rounded-lg border border-white/25 bg-white/10 px-4 py-2 text-sm font-semibold text-slate-200 disabled:opacity-50"
          >
            {L("登録を取り消す")}
          </button>
        ) : null}
      </div>
    </section>
  );
}
