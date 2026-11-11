"use client";

import { useCallback, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { AdminOverseasInterests, OverseasInterestKind } from "@/lib/types";

const kindLabels: Record<OverseasInterestKind, string> = {
  monthly: "1ヶ月コーチング",
  single: "単発の動画添削",
  both: "両方",
};

// 管理者画面の「海外希望」タブ。海外コーチの指導希望の集計（本体 #542）を見る。
// 総数・種別内訳・登録者の一覧。パイロットの個別案内に使う。
export function OverseasInterestAdmin({ token }: { token: string | null }) {
  const [data, setData] = useState<AdminOverseasInterests | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setData(await apiFetch<AdminOverseasInterests>("/v1/admin/overseas-interests", { token }));
      setError("");
    } catch {
      setError("読み込めませんでした");
    }
  }, [token]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (!data) return <p className="text-sm text-slate-200">読み込み中...</p>;

  return (
    <div className="space-y-3">
      <p className="text-sm text-slate-100">
        登録の総数：{data.total}件（1ヶ月コーチング {data.breakdown.monthly}件／単発の動画添削 {data.breakdown.single}件／両方 {data.breakdown.both}件）
      </p>
      {data.interests.length === 0 ? (
        <p className="text-sm text-slate-200">登録者はまだいません</p>
      ) : (
        <ul className="space-y-2">
          {data.interests.map((item, index) => (
            <li key={`${item.created_at}-${index}`} className="rounded-xl border border-white/20 bg-white/10 p-3 text-sm text-slate-100">
              {item.nickname || "名前なし"}（{kindLabels[item.kind]}、{item.created_at}）
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
