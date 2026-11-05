"use client";

import { PageLoading } from "@/components/page-loading";

import { GuardianConsultationReviews } from "@/components/guardian-consultation-reviews";

import { GuardianPersonalSection } from "@/components/coaching-guardian";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { GuardianPublic } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useParams } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

const relationships = [
  { value: "father", label: "父" },
  { value: "mother", label: "母" },
  { value: "other", label: "その他の保護者" },
] as const;

type Relationship = (typeof relationships)[number]["value"];

// このページは、サポートプラン（パーソナル指導など、保護者が契約してお支払いする指導）の保護者の入口。
// 動画添削・追加相談は、お子さまが購入のたびに宣言して自分で払う仕組みに変えたので、このページでは扱わない
// （前の仕組みの月の上限・購入の通知の説明と設定は外した。記録は消さず、保管期間の管理下に残す）。

export default function GuardianPage() {
  const { token } = useParams<{ token: string }>();
  const [consent, setConsent] = useState<GuardianPublic | null>(null);
  const [loadError, setLoadError] = useState("");
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState<Relationship | "">("");
  const [allowPublish, setAllowPublish] = useState(false);
  const [agree, setAgree] = useState(false);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const { pending, acquire, release } = useActionLock();

  function apply(next: GuardianPublic) {
    setConsent(next);
    setAllowPublish(next.allow_publish);
    if (next.guardian_name) setName(next.guardian_name);
    if (next.relationship) setRelationship(next.relationship);
  }

  useEffect(() => {
    let cancelled = false;
    apiFetch<GuardianPublic>(`/v1/guardian/${encodeURIComponent(token)}`)
      .then((data) => {
        if (!cancelled) apply(data);
      })
      .catch((err) => {
        if (!cancelled) setLoadError(err instanceof ApiRequestError ? err.message : "読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function send(path: string, method: string, body?: unknown, done?: string) {
    if (!acquire()) return;
    setError("");
    setFields({});
    setNotice("");
    try {
      const next = await apiFetch<GuardianPublic>(`/v1/guardian/${encodeURIComponent(token)}${path}`, {
        method,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      apply(next);
      if (done) setNotice(done);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setFields(err.fields ?? {});
        setError(err.fields ? "" : err.message);
      } else {
        setError("送信に失敗しました");
      }
    } finally {
      release();
    }
  }

  function onConsent(event: FormEvent) {
    event.preventDefault();
    void send(
      "/consent",
      "POST",
      // サポートプランは保護者がプランごとに同意してお支払いするので、月の上限は設けない（null は上限なし）。
      { guardian_name: name.trim(), relationship, monthly_limit_yen: null, allow_publish: allowPublish, agree },
      "同意を受け付けました。コーチがお子さまのお申し込みを引き受けると、このページの「パーソナル指導」に表示されます。プランごとに内容を確認して、同意とお支払いをしてください。",
    );
  }

  function onUpdate(event: FormEvent) {
    event.preventDefault();
    void send("", "PATCH", { allow_publish: allowPublish }, "設定を変更しました。");
  }

  function onRevoke() {
    if (!confirm("同意を取り消すと、このページから、お子さまのサポートプランへの同意やお支払いができなくなります。取り消しますか？")) return;
    void send("/revoke", "POST", undefined, "同意を取り消しました。");
  }

  function onDecline() {
    if (!confirm("同意しない場合、お子さまはサポートプランを利用できません。よろしいですか？")) return;
    void send("/decline", "POST", undefined, "「同意しない」を受け付けました。");
  }

  if (loadError) {
    return (
      <div className="mx-auto max-w-md space-y-3">
        <h1 className="text-2xl font-bold">保護者の方へ</h1>
        <p className="text-sm text-slate-300">{loadError}</p>
      </div>
    );
  }
  if (!consent) {
    return <PageLoading screen label="読み込み中..." />;
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">お子さまのサポートプラン利用についての同意のお願い</h1>
        <p className="text-sm text-slate-300">
          TrackTalkは、陸上競技の選手とコーチをつなぐサービスです。未成年（18歳未満）のお子さまがサポートプランを利用するには、保護者の方の同意が必要です。このページから同意いただくと、コーチが引き受けたお子さまのお申し込みについて、プランごとに内容を確認し、同意とお支払いができるようになります。
        </p>
      </div>

      {/* 文案（運営者の確認待ち）。確定したら、このコメントを外す。変えたら API の guardianTermsVersion を上げる（同意した版は terms_version で記録する）。 */}
      <section className="space-y-3 rounded-md border border-white/20 p-4 text-sm text-slate-200">
        <h2 className="font-bold text-white">同意いただく内容</h2>
        <ol className="list-decimal space-y-3 pl-5">
          <li>
            <p className="font-semibold text-white">サポートプランの利用について</p>
            <p>
              お子さまが、TrackTalkのサポートプラン（コーチによる動画添削・大会までのサポート・対面指導と、その追加相談）を利用することに同意します。これには、担当コーチとのサービス内でのメッセージのやりとりが含まれます。コーチとの連絡は、サービスの外（SNSや個人の連絡先など）では行わないルールです。
            </p>
          </li>
          <li>
            <p className="font-semibold text-white">プランごとの同意とお支払いについて</p>
            <p>
              サポートプランは、コーチが引き受けたお申し込みごとに、このページで保護者の方が内容を確認して同意してから始まります。お支払いがあるプランは、保護者の方がクレジットカードでお支払いいただき、お支払いが済んだ時から期間が始まります。
            </p>
          </li>
          <li>
            <p className="font-semibold text-white">同意の取り消しについて</p>
            <p>同意は、このページからいつでも取り消せます。取り消すと、このページからプランへの同意やお支払いはできなくなります。</p>
          </li>
        </ol>
        <p>
          同意にあたっては、TrackTalkの
          <a href="/terms" className="underline" target="_blank" rel="noreferrer">利用規約</a>
          および
          <a href="/privacy" className="underline" target="_blank" rel="noreferrer">プライバシーポリシー</a>
          もあわせてご確認ください。
        </p>
      </section>

      {notice ? <p className="rounded-md bg-emerald-500/15 p-3 text-sm text-emerald-200">{notice}</p> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      {consent.status === "consented" ? <><GuardianPersonalSection token={token} /><GuardianConsultationReviews token={token} /></> : null}

      {consent.status === "consented" ? (
        <form onSubmit={onUpdate} className="space-y-5">
          <p className="text-sm text-slate-200">
            {consent.guardian_name} さんが同意済みです。公開の可否は、このページからいつでも変えられます。
          </p>
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={allowPublish} onChange={(e) => setAllowPublish(e.target.checked)} className="mt-1" />
            添削の内容を、お子さまが選んだときに他の利用者の参考として公開することを認める（名前などは出ません）
          </label>
          <Button type="submit" className="w-full" disabled={pending}>
            変更を保存する
          </Button>
          <Button type="button" variant="ghost" className="w-full text-red-300" disabled={pending} onClick={onRevoke}>
            同意を取り消す
          </Button>
        </form>
      ) : !consent.can_consent ? (
        <p className="rounded-md border border-white/20 p-3 text-sm text-slate-200">
          {consent.status === "revoked" ? "同意は取り消されています。" : null}
          このリンクでは同意できません（同意に使えるのは、お子さまから届いた同意用のリンクで1回だけです）。もう一度同意する場合は、お子さまに新しいリンクをメールで送ってもらってください。
        </p>
      ) : (
        <form onSubmit={onConsent} className="space-y-5">
          {consent.status === "declined" ? (
            <p className="text-sm text-slate-300">「同意しない」が選ばれています。考えが変わったときは、ここから同意できます。</p>
          ) : null}
          {consent.status === "revoked" ? (
            <p className="text-sm text-slate-300">前の同意は取り消されています。もう一度同意する場合は、ここから入力してください。</p>
          ) : null}
          <label className="block space-y-1 text-sm">
            <span className="text-slate-300">保護者のお名前</span>
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={50} autoComplete="name" />
            {fields.guardian_name ? <span className="text-red-400">{fields.guardian_name}</span> : null}
          </label>
          <fieldset className="space-y-2">
            <legend className="text-sm text-slate-300">お子さまとの続柄</legend>
            <div className="flex flex-wrap gap-2">
              {relationships.map((option) => (
                <label
                  key={option.value}
                  className={`flex min-h-12 items-center gap-2 rounded-md border px-3 text-sm ${
                    relationship === option.value ? "border-blue-400 bg-blue-500/15 text-pure-white" : "border-white/25 text-slate-200"
                  }`}
                >
                  <input
                    type="radio"
                    name="relationship"
                    checked={relationship === option.value}
                    onChange={() => setRelationship(option.value)}
                  />
                  {option.label}
                </label>
              ))}
            </div>
            {fields.relationship ? <p className="text-sm text-red-400">{fields.relationship}</p> : null}
          </fieldset>
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={allowPublish} onChange={(e) => setAllowPublish(e.target.checked)} className="mt-1" />
            添削の内容を、お子さまが選んだときに他の利用者の参考として公開することを認める（任意。名前などは出ません）
          </label>
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} className="mt-1" />
            <span>上記の内容と、利用規約・プライバシーポリシーを確認し、お子さまのサポートプランの利用に同意します。</span>
          </label>
          {fields.agree ? <p className="text-sm text-red-400">{fields.agree}</p> : null}
          <Button type="submit" className="w-full" disabled={pending}>
            同意する
          </Button>
          {consent.status === "pending" ? (
            <Button type="button" variant="ghost" className="w-full" disabled={pending} onClick={onDecline}>
              同意しない
            </Button>
          ) : null}
        </form>
      )}
    </div>
  );
}
