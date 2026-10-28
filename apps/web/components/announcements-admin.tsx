"use client";

import { PageLoading } from "@/components/page-loading";

import { useCallback, useEffect, useState } from "react";
import { useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiErrorText, apiFetch } from "@/lib/api";
import { mainTabs } from "@/components/bottom-nav";
import { formatDateTime } from "@/lib/format";

type Announcement = {
  id: string;
  title: string;
  body: string;
  url: string;
  created_at: string;
  in_app_count: number;
  finished_at: string | null;
  push_recipients: number;
  canceled_at: string | null;
  sender_name: string | null;
};

type Recipients = { in_app: number; push: number; push_configured: boolean };

// 送信のとき入力してもらう言葉。API でも同じ言葉を確かめる。
const announcementConfirmWord = "送信";
const titleMax = 40;
const bodyMax = 200;

// 送信の確認を開いたときの中身。id は二度押しや再送で二重に送らないためのもの。
type Pending = { id: string; title: string; body: string; url: string; recipients: Recipients };

// 管理者画面の「お知らせ」タブ。全員のベルと、通知をオンにした人の端末へ、運営からのお知らせを送る。
// プッシュは届いたら取り消せないので、試し送信と、人数を見て「送信」と入力する確認を通す。
export function AnnouncementsAdmin({ token }: { token: string | null }) {
  const confirm = useConfirm();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [url, setURL] = useState("/");
  const [pending, setPending] = useState<Pending | null>(null);
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState<"" | "test" | "open" | "send" | "cancel">("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [history, setHistory] = useState<Announcement[] | null>(null);

  const load = useCallback(async () => {
    try {
      const res = await apiFetch<{ announcements: Announcement[] }>("/v1/admin/announcements", { token });
      setHistory(res.announcements);
    } catch {
      setError("送信の記録を読み込めませんでした");
    }
  }, [token]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const draft = { title: title.trim(), body: body.trim(), url: url.trim() || "/" };
  const filled = draft.title !== "" && draft.body !== "";

  async function sendTest() {
    setBusy("test");
    setError("");
    setMessage("");
    try {
      const res = await apiFetch<{ push_sent: number }>("/v1/admin/announcements/test", { method: "POST", token, body: JSON.stringify(draft) });
      setMessage(
        res.push_sent > 0
          ? `自分のベルと端末(${res.push_sent}台)に試し送信しました。見え方を確かめてください`
          : "自分のベルに試し送信しました。この端末で通知をオンにすると、プッシュの見え方も確かめられます",
      );
    } catch (err) {
      setError(apiErrorText(err, "試し送信できませんでした"));
    } finally {
      setBusy("");
    }
  }

  async function openConfirm() {
    setBusy("open");
    setError("");
    setMessage("");
    try {
      const recipients = await apiFetch<Recipients>("/v1/admin/announcements/recipients", { token });
      setTyped("");
      setPending({ id: crypto.randomUUID(), ...draft, recipients });
    } catch (err) {
      setError(apiErrorText(err, "送る人数を読み込めませんでした"));
    } finally {
      setBusy("");
    }
  }

  async function sendAll() {
    if (!pending || typed.trim() !== announcementConfirmWord || busy) return;
    setBusy("send");
    setError("");
    try {
      const res = await apiFetch<{ in_app: number; push_recipients: number }>("/v1/admin/announcements", {
        method: "POST",
        token,
        body: JSON.stringify({ id: pending.id, title: pending.title, body: pending.body, url: pending.url, confirm: typed.trim() }),
      });
      setMessage(`ベル ${res.in_app}人、プッシュ ${res.push_recipients}人に送りました`);
      setPending(null);
      setTitle("");
      setBody("");
      setURL("/");
    } catch (err) {
      setError(apiErrorText(err, "送信できませんでした。送信の記録を確かめてから、もう一度試してください"));
    } finally {
      setBusy("");
      void load();
    }
  }

  async function cancel(a: Announcement) {
    const ok = await confirm({
      title: "ベルからこのお知らせを消しますか？",
      body: "全員のベルから消えます。すでに届いたプッシュ通知は取り消せません。",
      confirmLabel: "ベルから消す",
    });
    if (!ok) return;
    setBusy("cancel");
    setError("");
    setMessage("");
    try {
      const res = await apiFetch<{ deleted: number }>(`/v1/admin/announcements/${a.id}/cancel`, { method: "POST", token });
      setMessage(`${res.deleted}人のベルから消しました`);
      await load();
    } catch (err) {
      setError(apiErrorText(err, "取り消せませんでした"));
    } finally {
      setBusy("");
    }
  }

  const label = "block space-y-1 text-sm text-slate-200";

  return (
    <div className="space-y-4">
      <section className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
        <h2 className="font-semibold">全員にお知らせを送る</h2>
        <p className="text-xs text-slate-300">
          全員のベルと、通知をオンにした人の端末に届きます(「運営からのお知らせ」をオフにした人を除く)。プッシュは届いたら取り消せないので、先に自分に試し送信してください。
        </p>
        <label className={label}>
          <span>見出し({title.trim().length}/{titleMax})</span>
          <Input value={title} maxLength={titleMax} disabled={pending !== null} onChange={(e) => setTitle(e.target.value)} placeholder="コーチ機能が使えるようになりました" />
        </label>
        <label className={label}>
          <span>本文({body.trim().length}/{bodyMax})</span>
          <Textarea rows={3} value={body} maxLength={bodyMax} disabled={pending !== null} onChange={(e) => setBody(e.target.value)} />
        </label>
        <fieldset disabled={pending !== null} className="space-y-2">
          <legend className="text-sm text-slate-200">タップしたときに開くページ</legend>
          <div className="grid grid-cols-5 gap-2" role="radiogroup" aria-label="お知らせをタップしたときに開くページ">
            {mainTabs.map(({ href, label, icon: Icon }) => (
              <label key={href} className={`flex cursor-pointer flex-col items-center gap-1 rounded-xl border p-2 text-xs ${url === href ? "border-blue-400 bg-blue-500/20 text-blue-200" : "border-white/20 text-slate-200"}`}>
                <input className="sr-only peer" type="radio" name="announcement-destination" value={href} checked={url === href} onChange={() => setURL(href)} />
                <Icon className="size-5 peer-focus-visible:outline peer-focus-visible:outline-2" aria-hidden />
                <span>{label}</span>
              </label>
            ))}
          </div>
        </fieldset>
        {pending ? null : (
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="secondary" disabled={!filled || busy !== ""} onClick={() => void sendTest()}>
              {busy === "test" ? "送信中..." : "自分にだけ試し送信"}
            </Button>
            <Button type="button" disabled={!filled || busy !== ""} onClick={() => void openConfirm()}>
              全員に送る...
            </Button>
          </div>
        )}
        {pending ? (
          <div role="alertdialog" aria-labelledby="announcement-confirm-title" className="space-y-3 rounded-xl border border-amber-400/60 bg-amber-500/10 p-3">
            <h3 id="announcement-confirm-title" className="font-semibold text-amber-200">
              この内容で全員に送りますか？
            </h3>
            <div className="rounded-lg border border-white/20 bg-slate-900/80 p-3">
              <p className="text-sm font-semibold text-white">{pending.title}</p>
              <p className="whitespace-pre-wrap text-sm text-slate-200">{pending.body}</p>
              <p className="mt-1 text-xs text-slate-400">タップで開く: {mainTabs.find((tab) => tab.href === pending.url)?.label ?? pending.url}</p>
            </div>
            <p className="text-sm text-slate-100">
              ベル: <strong>{pending.recipients.in_app}人</strong> ／ プッシュ: <strong>{pending.recipients.push}人</strong>
              {pending.recipients.push_configured ? "" : "(プッシュは未設定のため送りません)"}
            </p>
            <p className="text-xs text-amber-200">プッシュ通知は送ったら取り消せません。ベルの分はあとから消せます。</p>
            <label className={label}>
              <span>確認のため「{announcementConfirmWord}」と入力してください</span>
              <Input value={typed} disabled={busy === "send"} onChange={(e) => setTyped(e.target.value)} autoComplete="off" />
            </label>
            {busy === "send" ? <p className="text-sm text-slate-200">送信中です。終わるまでこの画面を閉じないでください。</p> : null}
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="danger" disabled={typed.trim() !== announcementConfirmWord || busy !== ""} onClick={() => void sendAll()}>
                {busy === "send" ? "送信中..." : "全員に送信"}
              </Button>
              <Button type="button" variant="secondary" disabled={busy === "send"} onClick={() => setPending(null)}>
                やめる
              </Button>
            </div>
          </div>
        ) : null}
        {message ? <p className="text-sm text-emerald-300">{message}</p> : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </section>

      <section className="space-y-2">
        <h2 className="font-semibold">送信の記録</h2>
        {history === null ? (
          <PageLoading label="読み込み中..." />
        ) : history.length === 0 ? (
          <p className="text-sm text-slate-300">まだ送っていません</p>
        ) : (
          <ul className="space-y-2">
            {history.map((a) => (
              <li key={a.id} className="space-y-1 rounded-xl border border-white/20 glass bg-white/10 p-3">
                <p className="text-sm font-semibold">{a.title}</p>
                <p className="whitespace-pre-wrap text-sm text-slate-200">{a.body}</p>
                <p className="text-xs text-slate-400">
                  {formatDateTime(a.created_at)} ・ {a.sender_name ?? "退会した管理者"} ・ 開く: {a.url}
                </p>
                <p className="text-xs text-slate-300">
                  ベル {a.in_app_count}人 ／ プッシュ {a.finished_at ? `${a.push_recipients}人` : "送信中または中断"}
                  {a.canceled_at ? ` ・ ${formatDateTime(a.canceled_at)} にベルから取り消し済み` : ""}
                </p>
                {a.canceled_at ? null : (
                  <Button type="button" variant="ghost" className="px-2 py-1 text-xs" disabled={busy !== ""} onClick={() => void cancel(a)}>
                    ベルから取り消す
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
