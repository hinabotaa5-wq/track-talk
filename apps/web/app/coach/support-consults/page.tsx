"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { SupportConsultDetail } from "@/components/support-consult-detail";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { supportConsultStatusLabels, type SupportConsult } from "@/lib/support-consult";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// コーチに届いた無料相談。48時間以内に提案か辞退を返す。
export default function CoachSupportConsultsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [rows, setRows] = useState<SupportConsult[] | null>(null);
  const [openID, setOpenID] = useState<string | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/coach/support-consults")}`);
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ consults: SupportConsult[] }>("/v1/coach/me/support-consults", { token })
      .then((d) => { if (!cancelled) setRows(d.consults); })
      .catch(() => { if (!cancelled) setError(L("無料相談を読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);

  if (loading || !session || (rows === null && !error)) return <PageLoading screen label={L("読み込み中...")} />;

  // 詳細（質問・提案つき）は開いたときに取る。一覧に出す形と同じ id で置き換える。
  async function open(id: string) {
    setOpenID(openID === id ? null : id);
    if (openID === id) return;
    try {
      const d = await apiFetch<{ consult: SupportConsult }>(`/v1/support-consults/${id}`, { token });
      setRows((r) => r?.map((x) => (x.id === id ? { ...x, ...d.consult } : x)) ?? r);
    } catch {
      setError(L("相談を読み込めませんでした"));
    }
  }

  return (
    <CoachSubpageShell title={L("無料相談")}>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {rows?.length === 0 ? <p className="text-sm text-slate-200">{L("届いている相談はありません。受付設定は継続プランにあります。")}</p> : null}
      <ul className="space-y-2">
        {rows?.map((c) => (
          <li key={c.id} className="rounded-xl border border-white/20 bg-white/10 p-3">
            <button type="button" className="w-full text-left text-sm" aria-expanded={openID === c.id} onClick={() => void open(c.id)}>
              <span className="block font-semibold">{c.athlete_nickname || L("選手")}{c.minor ? L("（未成年）") : ""}</span>
              <span className="block text-slate-200">{supportConsultStatusLabels[c.status]}</span>
              <span className="block text-xs text-slate-300">{formatDateTime(c.created_at)}</span>
            </button>
            {openID === c.id ? (
              <div className="mt-3">
                <SupportConsultDetail consult={c} role="coach" token={token}
                  onChange={(next) => setRows((r) => r?.map((x) => (x.id === c.id ? { ...x, ...next } : x)) ?? r)} />
              </div>
            ) : null}
          </li>
        ))}
      </ul>
    </CoachSubpageShell>
  );
}
