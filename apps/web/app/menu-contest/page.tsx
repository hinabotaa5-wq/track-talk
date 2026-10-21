"use client";

import Link from "next/link";
import { Check, Crown, Flame } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { ErrorState } from "@/components/empty-state";
import { useNow } from "@/components/menu-contest-bar";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { formatDate } from "@/lib/format";
import {
  formatRemaining,
  menuContestDeadline,
  menuContestMaxLength,
  menuContestName,
  menuContestPlaceholder,
  remainingParts,
  winners,
  type MenuContestFinalist,
  type MenuContestResult,
  type MenuContestRound,
  type MenuContestState,
} from "@/lib/menu-contest";

function errorMessage(err: unknown, fallback: string) {
  if (err instanceof ApiRequestError) return err.fields?.body ?? err.message;
  return fallback;
}

function Countdown({ round, now }: { round: MenuContestRound; now: number }) {
  const deadline = menuContestDeadline(round);
  const parts = remainingParts(Date.parse(deadline.until), now);
  return (
    <section
      className={`rounded-2xl border p-4 text-center ${
        parts?.urgent ? "border-red-300/60 bg-red-500/15" : "border-orange-300/50 bg-orange-500/12"
      }`}
    >
      <p className="text-sm font-semibold text-slate-100">
        第{round.number}回 ・ {deadline.label}
      </p>
      <p className="mt-1 text-xs text-slate-200">{deadline.prefix}</p>
      <p className={`mt-0.5 text-3xl font-black tabular-nums ${parts?.urgent ? "text-red-200" : "text-orange-200"}`}>
        {parts ? formatRemaining(parts) : "締め切りました"}
      </p>
    </section>
  );
}

function EntryForm({ round, token, onSaved }: { round: MenuContestRound; token: string; onSaved: () => void }) {
  const [body, setBody] = useState(round.my_entry?.body ?? "");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const length = [...body.trim()].length;

  async function save() {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      await apiFetch("/v1/menu-contest/entry", { method: "PUT", token, body: JSON.stringify({ body }) });
      setMessage(round.my_entry ? "書き直しました" : "応募しました。締め切りまでは書き直せます");
      onSaved();
    } catch (err) {
      setError(errorMessage(err, "応募できませんでした"));
    } finally {
      setSaving(false);
    }
  }

  async function withdraw() {
    if (!confirm("応募を取り消しますか？")) return;
    setSaving(true);
    setError("");
    try {
      await apiFetch("/v1/menu-contest/entry", { method: "DELETE", token });
      setBody("");
      setMessage("応募を取り消しました");
      onSaved();
    } catch (err) {
      setError(errorMessage(err, "取り消せませんでした"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="font-bold">{round.my_entry ? "あなたの応募" : "今回いちばんエグかった練習は？"}</h2>
      <Textarea
        rows={5}
        value={body}
        maxLength={menuContestMaxLength}
        placeholder={menuContestPlaceholder}
        onChange={(e) => setBody(e.target.value)}
        aria-label="エグかった練習"
      />
      <div className="flex items-center justify-between gap-3">
        <span className={`text-xs ${length > menuContestMaxLength ? "text-red-300" : "text-slate-300"}`}>
          {length}/{menuContestMaxLength}
        </span>
        <div className="flex items-center gap-2">
          {round.my_entry ? (
            <Button variant="ghost" onClick={() => void withdraw()} disabled={saving}>
              取り消す
            </Button>
          ) : null}
          <Button onClick={() => void save()} disabled={saving || length === 0 || length > menuContestMaxLength}>
            {saving ? "送信中..." : round.my_entry ? "書き直す" : "応募する"}
          </Button>
        </div>
      </div>
      {message ? <p className="text-sm text-emerald-300">{message}</p> : null}
      {error ? <p className="text-sm text-red-300">{error}</p> : null}
      <p className="text-xs text-slate-300">1人1件、100文字まで。応募されたものの中から運営が3つ選び、それを皆さんに投票してもらいます。</p>
    </section>
  );
}

function FinalistCard({
  finalist,
  chosen,
  onVote,
  busy,
}: {
  finalist: MenuContestFinalist;
  chosen?: boolean;
  onVote?: () => void;
  busy?: boolean;
}) {
  return (
    <li
      className={`rounded-2xl border p-4 ${
        finalist.winner
          ? "border-amber-300/70 bg-amber-400/15"
          : chosen
            ? "border-blue-400 bg-blue-500/20"
            : "border-white/20 glass bg-white/10"
      }`}
    >
      {finalist.winner ? (
        <p className="mb-2 flex items-center gap-1 text-sm font-bold text-amber-200">
          <Crown className="size-4" aria-hidden />
          1位
        </p>
      ) : null}
      <p className="whitespace-pre-wrap text-base leading-relaxed text-slate-100">{finalist.body}</p>
      <div className="mt-3 flex items-center justify-between gap-3 text-sm">
        <span className="text-slate-300">
          {finalist.mine ? "あなたの応募" : finalist.author_name ?? "匿名"}
          {finalist.votes !== undefined ? ` ・ ${finalist.votes}票` : ""}
        </span>
        {onVote && !finalist.mine ? (
          <Button variant={chosen ? "primary" : "secondary"} onClick={onVote} disabled={busy || chosen}>
            {chosen ? (
              <>
                <Check className="mr-1 size-4" aria-hidden />
                投票済み
              </>
            ) : (
              "これに投票"
            )}
          </Button>
        ) : null}
      </div>
    </li>
  );
}

function Voting({ round, token, onVoted }: { round: MenuContestRound; token: string | null; onVoted: (id: string) => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function vote(id: string) {
    if (!token) return;
    setBusy(true);
    setError("");
    try {
      await apiFetch("/v1/menu-contest/vote", { method: "PUT", token, body: JSON.stringify({ entry_id: id }) });
      onVoted(id);
    } catch (err) {
      setError(errorMessage(err, "投票できませんでした"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="space-y-3">
      <h2 className="font-bold">運営が選んだトップ3</h2>
      <p className="text-sm text-slate-200">いちばんエグいと思う練習に1票。締め切りまでは入れ直せます。</p>
      {!token ? (
        <p className="text-sm text-slate-200">
          投票するには<Link href="/login?next=/menu-contest" className="text-blue-300 underline">ログイン</Link>してください。
        </p>
      ) : null}
      <ul className="space-y-3">
        {round.finalists.map((f) => (
          <FinalistCard
            key={f.id}
            finalist={f}
            chosen={round.my_vote === f.id}
            busy={busy}
            onVote={token ? () => void vote(f.id) : undefined}
          />
        ))}
      </ul>
      {error ? <p className="text-sm text-red-300">{error}</p> : null}
    </section>
  );
}

function LastResult({ result }: { result: MenuContestResult }) {
  const top = winners(result);
  const rest = result.finalists.filter((f) => !f.winner).sort((a, b) => (b.votes ?? 0) - (a.votes ?? 0));
  return (
    <section className="space-y-3">
      <h2 className="font-bold">第{result.number}回の結果</h2>
      {top.length === 0 ? <p className="text-sm text-slate-200">投票がなかったため、1位はありませんでした。</p> : null}
      <ul className="space-y-3">
        {[...top, ...rest].map((f) => (
          <FinalistCard key={f.id} finalist={f} />
        ))}
      </ul>
    </section>
  );
}

export default function MenuContestPage() {
  const { session, token, loading } = useAuth();
  const [state, setState] = useState<MenuContestState | null>(null);
  const [failed, setFailed] = useState(false);
  const now = useNow();

  const load = useCallback(async () => {
    try {
      setState(await apiFetch<MenuContestState>("/v1/menu-contest", { token }));
      setFailed(false);
    } catch {
      setFailed(true);
    }
  }, [token]);

  useEffect(() => {
    if (loading) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [loading, load]);

  // 締め切りをまたいだら、次の段階を取り直す。
  const round = state?.round ?? null;
  const deadline = round ? Date.parse(menuContestDeadline(round).until) : null;
  const passed = deadline !== null && now >= deadline;
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (passed) void load();
  }, [passed, load]);

  return (
    <div className="mx-auto max-w-2xl space-y-5">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-bold">
          <Flame className="size-6 text-orange-300" aria-hidden />
          {menuContestName}
        </h1>
        <p className="mt-2 text-base font-semibold text-slate-100">過去2週間で1番えぐかった練習を書いて、みんなで1位を決めよう！</p>
      </div>

      {failed ? (
        <ErrorState message="エグ練大会を読み込めませんでした。時間をおいてもう一度開いてください。" />
      ) : !state ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : (
        <>
          {round ? (
            <Countdown round={round} now={now} />
          ) : (
            <section className="rounded-2xl border border-white/20 glass bg-white/10 p-4 text-center text-sm text-slate-100">
              {state.next_starts_at ? `次は ${formatDate(state.next_starts_at)} から開催します。` : "いまは開催していません。"}
            </section>
          )}

          {round?.phase === "entry" ? (
            session && token ? (
              <EntryForm key={round.start} round={round} token={token} onSaved={() => void load()} />
            ) : (
              <section className="rounded-2xl border border-white/20 glass bg-white/10 p-4 text-sm text-slate-100">
                応募するには<Link href="/login?next=/menu-contest" className="text-blue-300 underline">ログイン</Link>してください。
              </section>
            )
          ) : null}

          {round?.phase === "picking" ? (
            <section className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-4 text-sm text-slate-100">
              <p>応募は締め切りました。運営がトップ3を選んでいます。金曜から投票できます。</p>
              {round.my_entry ? (
                <p className="whitespace-pre-wrap rounded-xl bg-white/8 p-3 text-slate-200">あなたの応募: {round.my_entry.body}</p>
              ) : null}
            </section>
          ) : null}

          {round?.phase === "voting" ? (
            <Voting
              round={round}
              token={session ? token : null}
              onVoted={(id) => setState((s) => (s?.round ? { ...s, round: { ...s.round, my_vote: id } } : s))}
            />
          ) : null}

          {state.last_result ? <LastResult result={state.last_result} /> : null}

          <div className="space-y-1 text-sm text-slate-200">
            <p>投票は2週目の金曜から始まります。</p>
            <p className="text-xs text-slate-300">ケガをしているときや体調が悪いときは、無理な練習はしないでね。</p>
          </div>
        </>
      )}
    </div>
  );
}
