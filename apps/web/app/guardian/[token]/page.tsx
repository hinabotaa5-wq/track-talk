"use client";

import { GuardianPersonalSection } from "@/components/coaching-guardian";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { monthlyLimitLabel } from "@/lib/push";
import type { GuardianLimitYen, GuardianPublic } from "@/lib/types";
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
      <legend className="text-sm text-slate-300">月の上限（毎月1日にリセットされます）</legend>
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

export default function GuardianPage() {
  const { token } = useParams<{ token: string }>();
  const [consent, setConsent] = useState<GuardianPublic | null>(null);
  const [loadError, setLoadError] = useState("");
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState<Relationship | "">("");
  const [limit, setLimit] = useState<GuardianLimitYen>(3000);
  const [allowPublish, setAllowPublish] = useState(false);
  const [agree, setAgree] = useState(false);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const { pending, acquire, release } = useActionLock();

  function apply(next: GuardianPublic) {
    setConsent(next);
    setLimit(next.status === "consented" ? next.monthly_limit_yen : (next.default_limit_yen as GuardianLimitYen));
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
      { guardian_name: name.trim(), relationship, monthly_limit_yen: limit, allow_publish: allowPublish, agree },
      "同意を受け付けました。ありがとうございます。",
    );
  }

  function onUpdate(event: FormEvent) {
    event.preventDefault();
    void send("", "PATCH", { monthly_limit_yen: limit, allow_publish: allowPublish }, "設定を変更しました。");
  }

  function onRevoke() {
    if (!window.confirm("同意を取り消すと、お子さんは新しい添削を頼めなくなります。取り消しますか？")) return;
    void send("/revoke", "POST", undefined, "同意を取り消しました。");
  }

  function onDecline() {
    if (!window.confirm("同意しない場合、お子さんは添削を使えなくなります。よろしいですか？")) return;
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
        <h1 className="text-2xl font-bold">フォーム添削の保護者同意</h1>
        <p className="text-sm text-slate-300">
          お子さんが TrackTalk のフォーム添削を使うために、このページをお送りしました。TrackTalk は陸上競技をする人のための匿名のコミュニティです。
        </p>
      </div>

      <section className="space-y-2 rounded-md border border-white/20 p-4 text-sm text-slate-200">
        <h2 className="font-bold text-white">フォーム添削とは</h2>
        <ul className="list-disc space-y-1 pl-5">
          <li>お子さんが練習の動画（30秒まで）を送ると、運営が身元と競技実績を確認したコーチが、フォームの改善点を文章で返します。</li>
          <li>料金は1回ごとのお支払いです。月額料金や前払いはありません。</li>
          <li>やりとりは一方通行です。コーチとお子さんが連絡先を交換したり、直接会ったりすることはできません。</li>
          <li>動画は非公開で、担当のコーチと運営だけが見られます。</li>
          <li>最初の1回は同意の前に始まっていることがあります。2回目からは、この同意が必要です。</li>
        </ul>
      </section>

      {notice ? <p className="rounded-md bg-emerald-500/15 p-3 text-sm text-emerald-200">{notice}</p> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      {consent.status === "consented" ? <GuardianPersonalSection token={token} /> : null}

      {consent.status === "consented" ? (
        <form onSubmit={onUpdate} className="space-y-5">
          <p className="text-sm text-slate-200">
            {consent.guardian_name} さんが同意済みです。月の上限と公開の可否は、このページからいつでも変えられます。
          </p>
          <LimitChoices options={consent.limit_options} value={limit} onChange={setLimit} />
          {fields.monthly_limit_yen ? <p className="text-sm text-red-400">{fields.monthly_limit_yen}</p> : null}
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
      ) : (
        <form onSubmit={onConsent} className="space-y-5">
          {consent.status === "declined" ? (
            <p className="text-sm text-slate-300">「同意しない」が選ばれています。考えが変わったときは、ここから同意できます。</p>
          ) : null}
          {consent.status === "revoked" ? (
            <p className="text-sm text-slate-300">同意は取り消されています。もう一度同意する場合は、ここから入力してください。</p>
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
          <LimitChoices options={consent.limit_options} value={limit} onChange={setLimit} />
          {fields.monthly_limit_yen ? <p className="text-sm text-red-400">{fields.monthly_limit_yen}</p> : null}
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={allowPublish} onChange={(e) => setAllowPublish(e.target.checked)} className="mt-1" />
            添削の内容を、お子さんが選んだときに他の利用者の参考として公開することを認める（任意。名前などは出ません）
          </label>
          <label className="flex items-start gap-2 text-sm text-slate-200">
            <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} className="mt-1" />
            <span>
              上の内容と
              <a href="/terms" className="underline" target="_blank" rel="noreferrer">
                利用規約
              </a>
              を確認し、お子さんがフォーム添削を使うことに同意します
            </span>
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
