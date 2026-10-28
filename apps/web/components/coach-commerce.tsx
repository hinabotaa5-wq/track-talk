"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, apiErrorText } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { CoachCommerce, CoachIdentity, PublicCommerce } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useEffect, useState, type FormEvent } from "react";

// サポートプランはコーチが販売者なので、コーチごとに特定商取引法の表記を出す。
// 氏名・住所・電話番号は全コーチがプランの公開前に登録する。表記不要と申告したコーチの分は表示しない。

const emptyIdentity: CoachIdentity = { legal_name: "", address: "", phone: "" };

export function CoachCommerceSection({ token }: { token: string | null }) {
  const [data, setData] = useState<CoachCommerce | null>(null);
  const [identity, setIdentity] = useState<CoachIdentity>(emptyIdentity);
  const [checked, setChecked] = useState<boolean[]>([]);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();

  function apply(d: CoachCommerce) {
    setData(d);
    setIdentity(d.identity ?? emptyIdentity);
    setChecked(d.checklist.map(() => !d.required));
  }

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachCommerce>("/v1/coach/me/commerce", { token })
      .then((d) => { if (!cancelled) apply(d); })
      .catch((e) => { if (!cancelled) setError(apiErrorText(e, "特定商取引法の表記を読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);

  async function send(path: string, body: unknown, done: string) {
    if (!acquire()) return;
    setError(""); setMessage("");
    try {
      apply(await apiFetch<CoachCommerce>(path, { method: "PUT", token, body: JSON.stringify(body) }));
      setMessage(done);
    } catch (e) {
      setError(apiErrorText(e, "保存できませんでした"));
    } finally {
      release();
    }
  }

  function onSaveIdentity(e: FormEvent) {
    e.preventDefault();
    void send("/v1/coach/me/commerce/identity", identity, "登録しました");
  }

  if (!data) return error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null;
  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4" aria-label="特定商取引法の表記">
      <h2 className="font-bold">特定商取引法の表記</h2>
      <p className="text-sm text-slate-300">
        サポートプランは、あなたが販売者になります。プランを公開する前に、氏名・住所・電話番号を登録してください。表記が要る間は、プランのページにこの3つがそのまま表示されます。
      </p>
      <form onSubmit={onSaveIdentity} className="space-y-2">
        <label className="block text-sm">氏名（本名）<Input value={identity.legal_name} maxLength={80} onChange={(e) => setIdentity({ ...identity, legal_name: e.target.value })} aria-label="氏名" /></label>
        <label className="block text-sm">住所<Input value={identity.address} maxLength={200} onChange={(e) => setIdentity({ ...identity, address: e.target.value })} aria-label="住所" /></label>
        <label className="block text-sm">電話番号<Input value={identity.phone} maxLength={20} inputMode="tel" onChange={(e) => setIdentity({ ...identity, phone: e.target.value })} aria-label="電話番号" /></label>
        <p className="text-xs text-slate-300">暗号化して保存します。表記不要と申告した場合は表示せず、開示請求と紛争の対応にだけ使います。コーチをやめたり退会したりしても、最後の取引とやめた時の遅いほうから5年間は保管し、その後に削除します。</p>
        <Button type="submit" disabled={pending}>{data.identity ? "登録内容を更新する" : "登録する"}</Button>
      </form>

      <div className="space-y-2 border-t border-white/15 pt-3">
        <p className="text-sm font-semibold">表記の要否：{data.required ? "表記する" : "表記しない（申告済み）"}</p>
        {data.forced ? (
          <p className="text-xs text-amber-200">運営が表記要にしました。変える場合はお問い合わせください。</p>
        ) : data.required ? (
          <>
            <p className="text-xs text-slate-300">事業としての販売にあたらない場合だけ、表記を不要にできます。次のすべてに当てはまるときにチェックしてください。</p>
            {data.checklist.map((item, i) => (
              <label key={item} className="flex items-start gap-2 text-sm">
                <input type="checkbox" className="mt-1" checked={checked[i] ?? false} onChange={(e) => setChecked(checked.map((c, j) => (j === i ? e.target.checked : c)))} />
                <span>{item}</span>
              </label>
            ))}
            <Button type="button" disabled={pending || !checked.every(Boolean)} onClick={() => void send("/v1/coach/me/commerce/exemption", { exempt: true, checklist_version: data.checklist_version, checked }, "表記不要と申告しました")}>
              表記不要と申告する
            </Button>
          </>
        ) : (
          <>
            <p className="text-xs text-slate-300">{data.exempt_declared_at ? `${formatDateTime(data.exempt_declared_at)} に申告しました。` : null}事業の実態があると運営が判断した場合は、表記要に戻します。</p>
            <Button type="button" disabled={pending} onClick={() => void send("/v1/coach/me/commerce/exemption", { exempt: false }, "表記する設定に戻しました")}>表記する設定に戻す</Button>
          </>
        )}
      </div>
      {message ? <p role="status" className="text-sm text-blue-200">{message}</p> : null}
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}

// プランのページに出すコーチの表記。価格・期間・返金の条件はプランの内容をそのまま使う。
export function CoachCommerceDisclosure({ commerce }: { commerce: PublicCommerce }) {
  if (!commerce.required) {
    return <p className="text-xs leading-5 text-slate-300">このコーチは、事業としての販売にあたらないと申告しているため、特定商取引法に基づく表記はありません。</p>;
  }
  if (!commerce.registered) {
    return <p className="text-xs text-slate-300">このコーチの特定商取引法に基づく表記は準備中です。</p>;
  }
  return (
    <details className="rounded-lg border border-white/20 bg-white/5 p-2 text-sm">
      <summary className="cursor-pointer font-semibold">特定商取引法に基づく表記（販売者: コーチ）</summary>
      <dl className="mt-2 space-y-1">
        <div><dt className="text-xs text-slate-300">販売者</dt><dd>{commerce.legal_name}</dd></div>
        <div><dt className="text-xs text-slate-300">住所</dt><dd>{commerce.address}</dd></div>
        <div><dt className="text-xs text-slate-300">電話番号</dt><dd>{commerce.phone}</dd></div>
        <div><dt className="text-xs text-slate-300">販売価格・提供期間・返金の条件</dt><dd>このページのプランの内容のとおりです。</dd></div>
        <div><dt className="text-xs text-slate-300">支払方法・支払時期</dt><dd>コーチが申し込みを承諾したあと、TrackTalk の決済画面で支払います。<span className="ml-1 rounded bg-amber-500/20 px-1.5 py-0.5 text-xs text-amber-200">要確認</span></dd></div>
      </dl>
    </details>
  );
}
