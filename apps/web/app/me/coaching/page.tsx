"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { TicketBalance } from "@/components/ticket-balance";
import type { TicketSummary } from "@/lib/coaching-tickets";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { CoachingGuardian, CoachingMe } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useEffect, useState, type FormEvent } from "react";

// フォーム添削を始める前の準備（生まれた年月、保護者の同意、通知）。
// 添削の画面ができるまではメニューから辿れない。

const guardianLabels: Record<CoachingGuardian["status"], string> = {
  get pending() { return L("保護者の同意待ち"); },
  get consented() { return L("保護者が同意済み"); },
  get declined() { return L("保護者が同意しませんでした"); },
  get revoked() { return L("保護者が同意を取り消しました"); },
};

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiRequestError ? err.message : fallback;
}

export default function CoachingSetupPage() {
  const { session, token, loading } = useAuth();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const [month, setMonth] = useState("");
  const [monthError, setMonthError] = useState("");
  const [guardianEmail, setGuardianEmail] = useState("");
  const [guardianEmailError, setGuardianEmailError] = useState("");
  const [linkMessage, setLinkMessage] = useState("");
  const { pending, acquire, release } = useActionLock();
  // チケットの残りと有効期限は、マイページに常に出す（本体 #569）。読めなかった時は出さない。
  const [tickets, setTickets] = useState<TicketSummary | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token })
      .then((data) => { if (!cancelled) setTickets(data); })
      .catch(() => { if (!cancelled) setTickets(null); });
    return () => { cancelled = true; };
  }, [token]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((data) => {
        if (!cancelled) setMe(data);
      })
      .catch((err) => {
        if (!cancelled) setLoadError(errorMessage(err, L("読み込みに失敗しました")));
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function onBirthMonth(event: FormEvent) {
    event.preventDefault();
    if (!month || !acquire()) return;
    setMonthError("");
    try {
      setMe(
        await apiFetch<CoachingMe>("/v1/me/birth-month", {
          method: "PUT",
          token,
          body: JSON.stringify({ birth_month: month }),
        }),
      );
    } catch (err) {
      setMonthError(err instanceof ApiRequestError ? err.fields?.birth_month || err.message : L("登録に失敗しました"));
    } finally {
      release();
    }
  }

  // 保護者が一度答えたあとは、前と同じアドレスにしか送れない。
  const guardianEmailLocked = Boolean(me?.guardian && me.guardian.status !== "pending" && me.guardian.guardian_email);

  // 同意のリンクは API が保護者のメールあてに送る。本人の画面にはリンクを出さない（本人だけで同意できないように）。
  // アドレスを変えられないときは、省いて前と同じアドレスに送り直す。
  async function onSendLink(event: FormEvent) {
    event.preventDefault();
    if (!acquire()) return;
    setLinkMessage("");
    setGuardianEmailError("");
    try {
      const guardian = await apiFetch<CoachingGuardian>("/v1/me/guardian-link", {
        method: "POST",
        token,
        body: JSON.stringify(guardianEmailLocked ? {} : { guardian_email: guardianEmail.trim() }),
      });
      setMe((prev) => (prev ? { ...prev, guardian } : prev));
      setGuardianEmail("");
      setLinkMessage(L("{v0} にメールを送りました。届いたメールのリンクから、保護者が同意します。", { v0: guardian.guardian_email ?? L("保護者") }));
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields?.guardian_email) {
        setGuardianEmailError(err.fields.guardian_email);
      } else {
        setLinkMessage(errorMessage(err, L("いまは保護者にメールを送れません。しばらくしてからお試しください")));
      }
    } finally {
      release();
    }
  }

  if (loading || !session) {
    return <PageLoading screen label={L("読み込み中...")} />;
  }
  if (loadError) {
    return <p className="text-sm text-red-400">{loadError}</p>;
  }
  if (!me) {
    return <PageLoading screen label={L("読み込み中...")} />;
  }

  return (
    <div className="mx-auto max-w-md space-y-8">
      <section className="space-y-2">
        {tickets ? <TicketBalance summary={tickets} /> : null}
      </section>
      <h1 className="text-2xl font-bold">{L("サポートプランの準備")}</h1>

      <section className="space-y-3">
        <h2 className="font-bold">{L("生まれた年月")}</h2>
        {me.birth_month ? (
          <p className="text-sm text-slate-200">{L("{v0}月（{v1}）。変更はお問い合わせからお願いします。", { v0: me.birth_month.replace("-", L("年")), v1: me.is_minor ? L("18歳未満") : L("18歳以上") })}</p>
        ) : (
          <form onSubmit={onBirthMonth} className="space-y-2">
            <p className="text-sm text-slate-300">{L("年齢確認用です。他の人には見えません。登録後は変更できません。")}</p>
            <Input type="month" value={month} onChange={(e) => setMonth(e.target.value)} aria-label={L("生まれた年月")} />
            {monthError ? <p className="text-sm text-red-400">{monthError}</p> : null}
            <Button type="submit" className="w-full" disabled={!month || pending}>{L("登録する")}</Button>
          </form>
        )}
      </section>

      {me.is_minor ? (
        <section className="space-y-3">
          <h2 className="font-bold">{L("保護者の同意（パーソナル指導用）")}</h2>
          <p className="text-sm text-slate-200">{L("動画添削にはこの手続きは不要です。")}</p>
          <p className="text-sm text-slate-200">
            {me.guardian ? guardianLabels[me.guardian.status] : L("まだ保護者にメールを送っていません")}
          </p>
          <p className="text-sm text-slate-300">{L("保護者のメールアドレスを入力してください（自分のアドレスは不可）。")}</p>
          {me.guardian?.guardian_email ? (
            <p className="text-sm text-slate-300">{L("送り先: {guardian_email}", { guardian_email: me.guardian.guardian_email })}</p>
          ) : null}
          <form onSubmit={onSendLink} className="space-y-2">
            {guardianEmailLocked ? (
              <p className="text-xs text-slate-400">{L("保護者が一度答えたあとは、送り先のアドレスは変えられません。変えたいときはお問い合わせください。")}</p>
            ) : (
              <>
                <Input
                  type="email"
                  autoComplete="off"
                  value={guardianEmail}
                  onChange={(e) => setGuardianEmail(e.target.value)}
                  placeholder={me.guardian?.guardian_email ?? "parent@example.com"}
                  aria-label={L("保護者のメールアドレス")}
                />
                {guardianEmailError ? <p className="text-sm text-red-400">{guardianEmailError}</p> : null}
              </>
            )}
            <Button
              type="submit"
              variant="secondary"
              className="w-full"
              disabled={pending || (!guardianEmailLocked && !guardianEmail.trim())}
            >
              {me.guardian?.guardian_email ? L("保護者にメールを送り直す") : L("保護者にメールを送る")}
            </Button>
          </form>
          {me.guardian ? (
            <p className="text-xs text-slate-400">{L("送り直すと、前に送ったリンクは使えなくなります。メールは1日5回まで送れます。")}</p>
          ) : null}
          {linkMessage ? <p className="text-sm text-slate-300">{linkMessage}</p> : null}
        </section>
      ) : null}
    </div>
  );
}
