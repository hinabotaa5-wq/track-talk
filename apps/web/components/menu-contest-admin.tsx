"use client";

import { PageLoading } from "@/components/page-loading";

import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { formatDate, formatDateTime } from "@/lib/format";
import { menuContestDeadline, type MenuContestRound } from "@/lib/menu-contest";

type AdminEntry = {
  id: string;
  body: string;
  author_name: string | null;
  finalist: boolean;
  created_at: string;
  updated_at: string;
};

type AdminState = {
  round: MenuContestRound | null;
  entries: AdminEntry[];
  votes?: number;
  can_pick?: boolean;
  next_starts_at?: string;
};

const maxPicks = 3;

// 管理者画面の「エグ練」タブ。今の回の応募を全部読み、締め切り後にトップ3を選ぶ。
export function MenuContestAdmin({ token }: { token: string | null }) {
  const [state, setState] = useState<AdminState | null>(null);
  const [picked, setPicked] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const next = await apiFetch<AdminState>("/v1/admin/menu-contest", { token });
      setState(next);
      setPicked(next.entries.filter((e) => e.finalist).map((e) => e.id));
      setError("");
    } catch {
      setError("読み込めませんでした");
    }
  }, [token]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  function toggle(id: string) {
    setMessage("");
    setPicked((list) => (list.includes(id) ? list.filter((v) => v !== id) : list.length >= maxPicks ? list : [...list, id]));
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      await apiFetch("/v1/admin/menu-contest/finalists", { method: "PUT", token, body: JSON.stringify({ entry_ids: picked }) });
      setMessage("トップ3を保存しました。金曜から投票が始まります");
      await load();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.entry_ids ?? err.message : "保存できませんでした");
    } finally {
      setSaving(false);
    }
  }

  if (!state) return error ? <p className="text-sm text-red-400">{error}</p> : <PageLoading label="読み込み中..." />;

  const round = state.round;
  if (!round) {
    return (
      <p className="text-sm text-slate-200">
        いまは開催していません。{state.next_starts_at ? `次は ${formatDate(state.next_starts_at)} から始まります。` : ""}
      </p>
    );
  }

  const canPick = Boolean(state.can_pick);
  return (
    <div className="space-y-3">
      <div className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm text-slate-100">
        <p className="font-semibold">
          第{round.number}回 ・ {menuContestDeadline(round).label} ・ 応募 {state.entries.length}件
          {state.votes ? ` ・ 投票 ${state.votes}票` : ""}
        </p>
        <p className="mt-1 text-xs text-slate-300">
          応募の締め切り {formatDateTime(round.entry_ends_at)} ／ 投票 {formatDateTime(round.vote_starts_at)}〜{formatDateTime(round.vote_ends_at)}
        </p>
        <p className="mt-1 text-xs text-slate-300">
          {canPick
            ? "1〜3件を選んで保存してください。投票が1票でも入ると変えられません。"
            : round.phase === "entry"
              ? "応募の締め切り後に選べます。"
              : "投票が始まっているので、トップ3は変えられません。"}
        </p>
      </div>
      {state.entries.length === 0 ? <p className="text-sm text-slate-200">まだ応募がありません。</p> : null}
      <ul className="space-y-2">
        {state.entries.map((e) => {
          const on = picked.includes(e.id);
          return (
            <li key={e.id}>
              <label
                className={`flex gap-3 rounded-xl border p-3 ${on ? "border-blue-400 bg-blue-500/20" : "border-white/20 glass bg-white/10"} ${
                  canPick ? "cursor-pointer" : ""
                }`}
              >
                <input
                  type="checkbox"
                  className="mt-1 size-4 shrink-0"
                  checked={on}
                  disabled={!canPick || (!on && picked.length >= maxPicks)}
                  onChange={() => toggle(e.id)}
                />
                <span className="min-w-0">
                  <span className="block whitespace-pre-wrap text-sm text-slate-100">{e.body}</span>
                  <span className="mt-1 block text-xs text-slate-300">
                    {e.author_name ?? "ニックネームなし"} ・ {formatDateTime(e.updated_at)}
                  </span>
                </span>
              </label>
            </li>
          );
        })}
      </ul>
      {canPick ? (
        <div className="flex items-center gap-3">
          <Button onClick={() => void save()} disabled={saving || picked.length === 0}>
            {saving ? "保存中..." : `トップ3を保存(${picked.length}/${maxPicks})`}
          </Button>
          {message ? <span className="text-sm text-emerald-300">{message}</span> : null}
        </div>
      ) : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}
