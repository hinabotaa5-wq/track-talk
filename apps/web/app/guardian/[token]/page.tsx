"use client";

import { GuardianConsultationReviews } from "@/components/guardian-consultation-reviews";

import { GuardianPersonalSection } from "@/components/coaching-guardian";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { monthlyLimitLabel } from "@/lib/push";
import { formatYen } from "@/lib/format";
import type { GuardianLimitYen, GuardianPublic, SelfPayLimitYen } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useParams } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

const relationships = [
  { value: "father", label: "父" },
  { value: "mother", label: "母" },
  { value: "other", label: "その他の保護者" },
] as const;

type Relationship = (typeof relationships)[number]["value"];

function limitKey(value: GuardianLimitYen) {
  return value === null ? "none" : String(value);
}

function LimitChoices({
  options,
  value,
  onChange,
}: {
  options: GuardianLimitYen[];
  value: GuardianLimitYen;
  onChange: (value: GuardianLimitYen) => void;
}) {
  return (
    <fieldset className="space-y-2">
      <legend className="text-sm text-slate-300">保護者がお支払いする契約の月の上限（毎月1日にリセットされます）</legend>
      <p className="text-xs text-slate-300">パーソナル指導・サポートプランなど、保護者の方がお支払いする契約にかかる上限です。お子さんが自分で支払った分も合算します。</p>
      <div className="grid grid-cols-2 gap-2">
        {options.map((option) => (
          <label
            key={limitKey(option)}
            className={`flex min-h-12 items-center gap-2 rounded-md border px-3 text-sm ${
              value === option ? "border-blue-400 bg-blue-500/15 text-white" : "border-white/25 text-slate-200"
            }`}
          >
            <input
              type="radio"
              name="monthly_limit_yen"
              checked={value === option}
              onChange={() => onChange(option)}
            />
            {monthlyLimitLabel(option)}
          </label>
        ))}
      </div>
    </fieldset>
  );
}

// SelfPayLimitChoices は、お子さんが自分で支払う動画添削・追加相談の月の上限。上限なしは選べない。
function SelfPayLimitChoices({
  options,
  value,
  onChange,
}: {
  options: SelfPayLimitYen[];
  value: SelfPayLimitYen;
  onChange: (value: SelfPayLimitYen) => void;
}) {
  return (
    <fieldset className="space-y-2">
      <legend className="text-sm font-semibold text-white">お子さんが支払う月の上限（毎月1日にリセットされます）</legend>
      <p className="text-xs text-slate-300">
        動画添削（1回{formatYen(1500)}）と追加相談（1回{formatYen(600)}）の、その月の合計です。超える購入は止まり、保護者の方に確認のメールが届きます。
      </p>
      <div className="grid grid-cols-2 gap-2">
        {options.map((option) => (
          <label
            key={option}
            className={`flex min-h-12 items-center gap-2 rounded-md border px-3 text-sm ${
              value === option ? "border-blue-400 bg-blue-500/15 text-white" : "border-white/25 text-slate-200"
            }`}
          >
            <input type="radio" name="self_pay_limit_yen" checked={value === option} onChange={() => onChange(option)} />
            {monthlyLimitLabel(option)}
          </label>
        ))}
      </div>
    </fieldset>
  );
}

// ContractLimit は、保護者が払う契約（パーソナル指導など）の上限。使う人が少ないので畳んでおく。
function ContractLimit({
  options,
  value,
  onChange,
  error,
}: {
  options: GuardianLimitYen[];
  value: GuardianLimitYen;
  onChange: (value: GuardianLimitYen) => void;
  error?: string;
}) {
  return (
    <details className="rounded-md border border-white/20 p-3">
      <summary className="cursor-pointer text-sm text-slate-200">パーソナル指導など、保護者がお支払いする契約の上限（{monthlyLimitLabel(value)}）</summary>
      <div className="mt-3">
        <LimitChoices options={options} value={value} onChange={onChange} />
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </div>
    </details>
  );
}

export default function GuardianPage() {
  const { token } = useParams<{ token: string }>();
  const [consent, setConsent] = useState<GuardianPublic | null>(null);
  const [loadError, setLoadError] = useState("");
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState<Relationship | "">("");
  const [limit, setLimit] = useState<GuardianLimitYen>(3000);
  const [selfPayLimit, setSelfPayLimit] = useState<SelfPayLimitYen>(1500);
  const [allowPublish, setAllowPublish] = useState(false);
  const [agree, setAgree] = useState(false);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const { pending, acquire, release } = useActionLock();

  function apply(next: GuardianPublic) {
    setConsent(next);
    setLimit(next.status === "consented" ? next.monthly_limit_yen : (next.default_limit_yen as GuardianLimitYen));
    setSelfPayLimit(next.status === "consented" ? next.self_pay_limit_yen : next.default_self_pay_limit_yen);
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
      { guardian_name: name.trim(), relationship, self_pay_limit_yen: selfPayLimit, monthly_limit_yen: limit, allow_publish: allowPublish, agree },
      "同意を受け付けました。確認のメールをお送りしました。上限の変更と同意の取り消しは、このページかメールのリンクからできます。",
    );
  }

  function onUpdate(event: FormEvent) {
    event.preventDefault();
    void send("", "PATCH", { self_pay_limit_yen: selfPayLimit, monthly_limit_yen: limit, allow_publish: allowPublish }, "設定を変更しました。");
  }

  function onRevoke() {
    if (!confirm("同意を取り消すと、お子さんは新しい動画添削・追加相談を購入できなくなります（購入済みの分はそのまま使えます）。取り消しますか？")) return;
    void send("/revoke", "POST", undefined, "同意を取り消しました。");
  }

  function onDecline() {
    if (!confirm("同意しない場合、お子さんは動画添削・追加相談を購入できません。よろしいですか？")) return;
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
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">オンラインコーチングの保護者同意</h1>
        <p className="text-sm text-slate-300">
          お子さんが TrackTalk のオンラインコーチングを使うために、このページをお送りしました。TrackTalk は陸上競技をする人のための匿名のコミュニティです。
        </p>
      </div>

      {/* 同意の範囲の文面はたたき台。運営者が確定する。版は terms_version で記録する。 */}
      <section className="space-y-2 rounded-md border border-white/20 p-4 text-sm text-slate-200">
        <h2 className="font-bold text-white">同意いただく内容</h2>
        <ol className="list-decimal space-y-2 pl-5">
          <li>
            コーチングサービスの利用：お子さんが練習の動画（30秒まで）を送り、運営が身元と競技実績を確認したコーチが改善点を返す「動画添削」（1回{formatYen(1500)}）と、その後の「追加相談」（1回{formatYen(600)}）、コーチとのサービス内のメッセージ。
          </li>
          <li>お子さんが購入のたびに、お子さん自身の PayPay（またはカード）でお支払いすること。月額料金や前払いはありません。</li>
          <li>保護者の方が下で選ぶ「お子さんが支払う月の上限」。その月の合計が上限を超える購入はできません。</li>
          <li>
            <a href="/terms" className="underline" target="_blank" rel="noreferrer">利用規約</a>・
            <a href="/privacy" className="underline" target="_blank" rel="noreferrer">プライバシーポリシー</a>
            と安全面のルール：コーチとの連絡はサービス内だけで行い、連絡先の交換や直接会うことはできません。動画は非公開で、担当のコーチと運営だけが見られます。
          </li>
        </ol>
        <ul className="list-disc space-y-1 pl-5 text-xs text-slate-300">
          <li>購入があるたびに、金額・コーチ名・日時をこのメールアドレスにお知らせします（事前の承認ではなく、事後のお知らせです）。</li>
          <li>上限の変更と同意の取り消しは、このページとお知らせのメールのリンクからいつでもできます。取り消すと以後の購入は止まり、購入済みの分には影響しません。</li>
          <li>同意した日時・接続元の IP アドレス・同意した文面の版（{consent.terms_version}）を記録します。</li>
        </ul>
      </section>

      {notice ? <p className="rounded-md bg-emerald-500/15 p-3 text-sm text-emerald-200">{notice}</p> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      {consent.status === "consented" ? <><GuardianPersonalSection token={token} /><GuardianConsultationReviews token={token} /></> : null}

      {consent.status === "consented" ? (
        <form onSubmit={onUpdate} className="space-y-5">
          <p className="text-sm text-slate-200">
            {consent.guardian_name} さんが同意済みです。お子さんが支払う月の上限と公開の可否は、このページからいつでも変えられます。
          </p>
          <SelfPayLimitChoices options={consent.self_pay_limit_options} value={selfPayLimit} onChange={setSelfPayLimit} />
          {fields.self_pay_limit_yen ? <p className="text-sm text-red-400">{fields.self_pay_limit_yen}</p> : null}
          <ContractLimit options={consent.limit_options} value={limit} onChange={setLimit} error={fields.monthly_limit_yen} />
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={allowPublish} onChange={(e) => setAllowPublish(e.target.checked)} className="mt-1" />
            添削の内容を、お子さんが選んだときに他の利用者の参考として公開することを認める（名前などは出ません）
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
          このリンクでは同意できません（同意に使えるのは、お子さんから届いた同意用のリンクで1回だけです）。もう一度同意する場合は、お子さんに新しいリンクをメールで送ってもらってください。
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
            <legend className="text-sm text-slate-300">お子さんとの続柄</legend>
            <div className="flex flex-wrap gap-2">
              {relationships.map((option) => (
                <label
                  key={option.value}
                  className={`flex min-h-12 items-center gap-2 rounded-md border px-3 text-sm ${
                    relationship === option.value ? "border-blue-400 bg-blue-500/15 text-white" : "border-white/25 text-slate-200"
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
          <SelfPayLimitChoices options={consent.self_pay_limit_options} value={selfPayLimit} onChange={setSelfPayLimit} />
          {fields.self_pay_limit_yen ? <p className="text-sm text-red-400">{fields.self_pay_limit_yen}</p> : null}
          <ContractLimit options={consent.limit_options} value={limit} onChange={setLimit} error={fields.monthly_limit_yen} />
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={allowPublish} onChange={(e) => setAllowPublish(e.target.checked)} className="mt-1" />
            添削の内容を、お子さんが選んだときに他の利用者の参考として公開することを認める（任意。名前などは出ません）
          </label>
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} className="mt-1" />
            <span>上の「同意いただく内容」の1〜4と選んだ上限を確認し、お子さんがオンラインコーチングを使い、その範囲で自分で支払うことに同意します</span>
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
