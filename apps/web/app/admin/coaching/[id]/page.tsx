"use client";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { SuspendToggle } from "@/components/coaching-admin";
import { ContractDetails, contractPeriod } from "@/components/coaching-contracts";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabel } from "@/lib/coaching-plan";
import { formatDateTime } from "@/lib/format";
import type { CoachingAccessLog, CoachingContract, CoachingMessage } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

// 通報があったパーソナル指導のやりとりを、運営が開く。開くたびに、開いた人と日時が消せない形で残る。
// 開く前に確認をはさみ、ページを開いただけでは読まない。

type Opened = { contract: CoachingContract; messages: CoachingMessage[]; access_logs: CoachingAccessLog[] };

const roleLabels: Record<CoachingMessage["sender_role"], string> = { requester: "利用者", coach: "コーチ", guardian: "保護者" };

function senderLabel(m: CoachingMessage, c: CoachingContract): string {
  if (m.sender_role === "guardian") return "保護者";
  if (!m.sender_active) return `${roleLabels[m.sender_role]}（退会）`;
  const name = m.sender_role === "coach" ? c.coach?.nickname : c.requester?.nickname;
  return `${roleLabels[m.sender_role]} ${name ?? "名前なし"}`;
}

function OpenContract() {
  const { profile, token, loading } = useAuth();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const report = useSearchParams().get("report") ?? "";
  const [opened, setOpened] = useState<Opened | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !profile?.is_admin) router.replace("/");
  }, [loading, profile, router]);

  async function open() {
    if (!acquire()) return;
    setError("");
    try {
      setOpened(await apiFetch<Opened>(`/v1/admin/coaching/contracts/${id}?report=${encodeURIComponent(report)}`, { token }));
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "開けませんでした");
    } finally {
      release();
    }
  }

  if (loading || !profile?.is_admin) return <PageLoading label="読み込み中..." />;

  if (!opened) {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">パーソナル指導のやりとり</h1>
        <p className="text-sm text-slate-200">
          通報の確認のために、この契約のやりとりを開きます。開いた人と日時は記録され、あとから消せません。確認に必要なときだけ開いてください。
        </p>
        <button
          type="button"
          disabled={pending || !report}
          onClick={() => void open()}
          className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition active:scale-[0.98] disabled:opacity-50"
        >
          記録を残して開く
        </button>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Link href="/admin" className="block text-center text-sm text-blue-300 underline">
          アプリ管理へ
        </Link>
      </div>
    );
  }

  const c = opened.contract;
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">パーソナル指導のやりとり</h1>
        <p className="text-sm font-semibold text-blue-300">
          {contractStatusLabel(c)}
          {c.minor ? " · 18歳未満" : ""}
        </p>
      </div>
      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        <p className="text-sm text-white">
          {c.requester?.nickname ?? "名前なし"}さん → コーチ {c.coach?.nickname ?? "名前なし"}
        </p>
        <ContractDetails contract={c} />
        {c.starts_at ? <p className="text-sm text-slate-200">期間: {contractPeriod(c)}</p> : null}
        <SuspendToggle id={c.id} status={c.status} token={token} onChange={(status) => setOpened({ ...opened, contract: { ...c, status } })} />
      </section>
      <section className="space-y-2">
        <h2 className="font-bold">やりとり（{opened.messages.length}件）</h2>
        <ul className="space-y-2">
          {opened.messages.map((m) => (
            <li key={m.id} className="space-y-1 rounded-xl border border-white/20 bg-white/10 p-3 text-sm">
              <p className="text-xs text-slate-300">
                {senderLabel(m, c)} · {formatDateTime(m.created_at)}
              </p>
              {m.body ? <p className="whitespace-pre-wrap text-slate-100">{m.body}</p> : null}
              {m.video_url ? (
                <video src={m.video_url} poster={m.thumbnail_url || undefined} controls playsInline preload="metadata" className="w-full rounded-lg bg-black" />
              ) : null}
              {m.menu ? (
                <div className="rounded-lg border border-blue-400/40 p-2 text-xs text-slate-100">
                  <p className="font-semibold">メニュー: {m.menu.title}</p>
                  {m.menu.days.map((d, i) => (
                    <p key={i}>
                      {d.day}: {d.content}
                    </p>
                  ))}
                </div>
              ) : null}
            </li>
          ))}
        </ul>
      </section>
      <section className="space-y-2">
        <h2 className="font-bold">開いた記録</h2>
        <ul className="space-y-1 text-xs text-slate-300">
          {opened.access_logs.map((l) => (
            <li key={l.id}>
              {formatDateTime(l.opened_at)} · {l.admin_nickname ?? "名前なし"}
            </li>
          ))}
        </ul>
      </section>
      <Link href="/admin" className="block text-center text-sm text-blue-300 underline">
        アプリ管理へ
      </Link>
    </div>
  );
}

export default function AdminCoachingContractPage() {
  return (
    <Suspense fallback={<PageLoading label="読み込み中..." />}>
      <OpenContract />
    </Suspense>
  );
}
