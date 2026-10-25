"use client";

import { useEffect, useId, useState, type FormEvent } from "react";
import { Lock, Pencil, Trash2 } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { EmptyState, ErrorState } from "./empty-state";
import { apiFetch, apiErrorText } from "@/lib/api";
import { todayInTokyo } from "@/lib/format";
import { useActionLock } from "@/lib/use-action-lock";
import { deleteConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { senseDayLabel } from "@/lib/sense-notes";
import {
  formatRaceMark,
  formatRaceWind,
  parseRaceMark,
  parseRaceWind,
  toggleRaceWindSign,
  raceChartBox,
  raceChartPoints,
  raceChartDomain,
  raceOverviewSeries,
  sortRaceRecords,
  raceRoundSuggestions,
  raceRecordRoundMaxLength,
  raceEventHasWind,
  raceMarkInput,
  raceMarkKind,
  raceRecordEvents,
  raceRecordMeetNameMaxLength,
  raceRecordMemoMaxLength,
  sortRaceEvents,
  type RaceMarkKind,
  type RaceRecord,
  type RaceRecords,
} from "@/lib/race-records";

const selectClass =
  "w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400";

const markPlaceholder: Record<RaceMarkKind, string> = { time: "例: 10.85 / 1:58.32", distance: "例: 7.05", points: "例: 5800" };

// 記録を書く欄。新しく書くときと、直すときの両方で使う。
function RaceRecordForm({
  token,
  record,
  defaultEvent,
  defaultDay,
  onSaved,
  onCancel,
}: {
  token: string | null;
  record?: RaceRecord;
  defaultEvent: string;
  defaultDay?: string;
  onSaved: (records: RaceRecords, event: string) => void;
  onCancel?: () => void;
}) {
  const [day, setDay] = useState(record?.raced_on ?? defaultDay ?? todayInTokyo());
  const [event, setEvent] = useState(record?.event ?? defaultEvent);
  const kind = raceMarkKind(event);
  const [mark, setMark] = useState(record ? raceMarkInput(record.mark, kind) : "");
  const [wind, setWind] = useState(record?.wind != null ? formatRaceWind(record.wind).replace("±", "") : "");
  const roundListId = useId();
  const [round, setRound] = useState(record?.round ?? "");
  const [meetName, setMeetName] = useState(record?.meet_name ?? "");
  const [memo, setMemo] = useState(record?.memo ?? "");
  const [error, setError] = useState("");
  const lock = useActionLock();
  const hasWind = raceEventHasWind(event);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const parsedMark = parseRaceMark(mark, kind);
    if (parsedMark === null) {
      setError(kind === "time" ? "記録は 10.85 や 1:58.32 の形で入力してください" : kind === "distance" ? "記録は 7.05 のようにメートルで入力してください" : "記録は点数を数字で入力してください");
      return;
    }
    const parsedWind = hasWind ? parseRaceWind(wind) : undefined;
    if (parsedWind === null) {
      setError("風は +1.8 や -0.5 の形で入力してください");
      return;
    }
    if ([...memo.trim()].length > raceRecordMemoMaxLength) {
      setError(`感想は${raceRecordMemoMaxLength}文字までです`);
      return;
    }
    if (!lock.acquire()) return;
    setError("");
    try {
      const res = await apiFetch<RaceRecords>(record ? `/v1/me/race-records/${record.id}` : "/v1/me/race-records", {
        method: record ? "PATCH" : "POST",
        token,
        body: JSON.stringify({ raced_on: day, event, mark: parsedMark, wind: parsedWind ?? null, memo: memo.trim(), meet_name: meetName.trim(), round: round.trim() }),
      });
      if (!record) {
        setMark("");
        setWind("");
        setRound("");
        setMemo("");
      }
      onSaved(res, event);
    } catch (err) {
      setError(apiErrorText(err, "保存できませんでした"));
    } finally {
      lock.release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-2">
      <div className="grid grid-cols-2 gap-2">
        <label className="block">
          <span className="mb-1 block text-xs text-slate-300">日付</span>
          <Input type="date" value={day} max={todayInTokyo()} onChange={(e) => setDay(e.target.value)} required />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs text-slate-300">種目</span>
          <select className={selectClass} value={event} onChange={(e) => setEvent(e.target.value)}>
            {raceRecordEvents.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-xs text-slate-300">記録</span>
          <Input inputMode="decimal" value={mark} onChange={(e) => setMark(e.target.value)} placeholder={markPlaceholder[kind]} required />
        </label>
        {hasWind ? (
          <label className="block">
            <span className="mb-1 block text-xs text-slate-300">風(m/s)</span>
            <span className="flex gap-1">
              <Input inputMode="decimal" value={wind} onChange={(e) => setWind(e.target.value)} placeholder="例: +1.8" className="min-w-0 flex-1" />
              <button
                type="button"
                onClick={() => setWind((current) => toggleRaceWindSign(current))}
                aria-label="追い風(+)と向かい風(-)を切り替える"
                className="shrink-0 rounded-md border border-white/25 glass bg-white/10 px-3 text-sm font-semibold text-white transition active:scale-[0.98]"
              >
                +/−
              </button>
            </span>
          </label>
        ) : null}
      </div>
      <label className="block">
        <span className="mb-1 block text-xs text-slate-300">大会名</span>
        <Input value={meetName} onChange={(e) => setMeetName(e.target.value)} maxLength={raceRecordMeetNameMaxLength} placeholder="例: 県高校総体" />
      </label>
      <details open={record?.round ? true : undefined} className="rounded-lg border border-white/15 p-3">
        <summary className="cursor-pointer text-sm text-slate-200">予選・準決勝・決勝などを記入（任意）</summary>
        <label className="mt-3 block">
          <span className="mb-1 block text-xs text-slate-300">ラウンド（任意）</span>
          <Input value={round} onChange={(e) => setRound(e.target.value)} list={roundListId} maxLength={raceRecordRoundMaxLength} placeholder="例: 予選 / 準決勝 / 決勝" />
          <datalist id={roundListId}>{raceRoundSuggestions.map((value) => <option key={value} value={value} />)}</datalist>
        </label>
        <p className="mt-2 text-xs text-slate-300">記録会などは空欄のまま保存できます。同じ日・種目でも、1本ずつ記録を残せます。</p>
      </details>
      <label className="block">
        <span className="mb-1 block text-xs text-slate-300">感想</span>
        <Textarea value={memo} onChange={(e) => setMemo(e.target.value)} rows={2} maxLength={raceRecordMemoMaxLength + 20} placeholder="例: 後半で腕振りが小さくなった" />
      </label>
      {error ? <p className="text-sm text-red-300">{error}</p> : null}
      <div className="flex items-center justify-between gap-3">
        <p className="flex items-center gap-1 text-xs text-slate-300">
          <Lock className="size-3.5" aria-hidden />
          あなただけに見えます
        </p>
        <div className="flex gap-2">
          {onCancel ? (
            <Button type="button" variant="secondary" onClick={onCancel}>
              やめる
            </Button>
          ) : null}
          <Button type="submit" disabled={lock.pending}>
            {lock.pending ? "保存しています…" : record ? "直す" : "記録を残す"}
          </Button>
        </div>
      </div>
    </form>
  );
}

// 1つの種目の推移。よい記録ほど上。自己ベストは色を変え、追い風参考は白抜きにする。
function RaceRecordChart({ records, kind }: { records: RaceRecord[]; kind: RaceMarkKind }) {
  const points = raceChartPoints(records, kind);
  if (points.length === 0) return null;
  const box = raceChartBox;
  const ordered = sortRaceRecords(records);
  const marks = ordered.map((record) => record.mark);
  const { low, high } = raceChartDomain(marks);
  const topMark = kind === "time" ? low : high;
  const bottomMark = kind === "time" ? high : low;
  const first = ordered[0].raced_on;
  const last = ordered[ordered.length - 1].raced_on;
  const path = points.map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ");

  return (
    <figure className="space-y-1">
      <svg viewBox={`0 0 ${box.width} ${box.height}`} className="h-auto w-full" role="img" aria-label="記録の推移のグラフ">
        <line x1={box.left} x2={box.width - box.right} y1={box.top} y2={box.top} className="stroke-white/15" />
        <line x1={box.left} x2={box.width - box.right} y1={box.height - box.bottom} y2={box.height - box.bottom} className="stroke-white/25" />
        <line x1={box.left} x2={box.width - box.right} y1={(box.top + box.height - box.bottom) / 2} y2={(box.top + box.height - box.bottom) / 2} className="stroke-white/10" strokeDasharray="3 3" />
        {[topMark, (low + high) / 2, bottomMark].map((mark, i) => <text key={i} x={box.left - 6} y={box.top + i / 2 * (box.height - box.top - box.bottom) + 4} textAnchor="end" className="fill-slate-300 text-xs">{formatRaceMark(Math.round(mark), kind)}</text>)}
        <text x={box.left} y={box.height - 8} className="fill-slate-400 text-xs">
          {senseDayLabel(first)}
        </text>
        {first !== last ? (
          <text x={box.width - box.right} y={box.height - 8} textAnchor="end" className="fill-slate-400 text-xs">
            {senseDayLabel(last)}
          </text>
        ) : null}
        <path d={path} fill="none" className="stroke-sky-300/70" strokeWidth={1.5} strokeLinejoin="round" />
        {points.map((p) => (
          <circle
            key={p.record.id}
            cx={p.x}
            cy={p.y}
            r={p.record.is_pb ? 5 : 3.5}
            strokeWidth={1.5}
            className={p.record.wind_aided ? "fill-slate-950 stroke-slate-200" : p.record.is_pb ? "fill-amber-300 stroke-amber-100" : "fill-sky-300 stroke-sky-100"}
          >
            <title>{`${p.record.raced_on} ${p.record.meet_name} ${p.record.round ?? ""} ${formatRaceMark(p.record.mark, kind)}`}</title>
          </circle>
        ))}
      </svg>
      <figcaption className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-300">
        <span className="flex items-center gap-1">
          <span className="inline-block size-2.5 rounded-full bg-amber-300" aria-hidden />
          自己ベスト更新
        </span>
        {records.some((record) => record.wind_aided) ? (
          <span className="flex items-center gap-1">
            <span className="inline-block size-2.5 rounded-full border border-slate-200" aria-hidden />
            追い風参考
          </span>
        ) : null}
      </figcaption>
    </figure>
  );
}

function RaceOverviewChart({ records }: { records: RaceRecord[] }) {
  const { domain, series } = raceOverviewSeries(records);
  const box = raceChartBox;
  const sorted = sortRaceRecords(records);
  const first = sorted[0]?.raced_on, last = sorted.at(-1)?.raced_on;
  const y = (value: number) => box.top + (domain.high - value) / (domain.high - domain.low) * (box.height - box.top - box.bottom);
  return <figure className="space-y-3">
    <h3 className="text-sm font-semibold">全種目の伸びを比較</h3>
    <p className="text-xs leading-relaxed text-slate-300">各種目の最初の記録を100とし、タイムの短縮率・距離や得点の増加率を重ねています。上に行くほど初回よりよい記録です。種目間の実力や公式の得点を比較するものではありません。</p>
    <svg viewBox={`0 0 ${box.width} ${box.height}`} className="h-auto w-full" role="img" aria-label="全種目の初回の記録を100とした伸び率のグラフ">
      {[domain.high, 100, domain.low].map((value, i) => <g key={i}>
        <line x1={box.left} x2={box.width - box.right} y1={y(value)} y2={y(value)} className={value === 100 ? "stroke-white/40" : "stroke-white/15"} strokeDasharray={value === 100 ? "4 3" : undefined} />
        <text x={box.left - 6} y={y(value) + 4} textAnchor="end" className="fill-slate-300 text-xs">{value.toFixed(1)}</text>
      </g>)}
      {series.map((item) => <g key={item.event}>
        <path d={item.points.map((p, i) => `${i ? "L" : "M"}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ")} fill="none" stroke={item.color} strokeWidth={1.5} />
        {item.points.map((p) => <circle key={p.record.id} cx={p.x} cy={p.y} r={p.record.is_pb ? 4 : 3} fill={p.record.wind_aided ? "#020617" : item.color} stroke={item.color} strokeWidth={1.5}>
          <title>{`${p.record.raced_on} ${item.event_label} ${p.record.round ?? ""} ${formatRaceMark(p.record.mark, item.kind)} / ${p.index.toFixed(1)}${p.record.wind_aided ? "（追い風参考）" : ""}`}</title>
        </circle>)}
      </g>)}
      {first ? <text x={box.left} y={box.height - 8} className="fill-slate-400 text-xs">{senseDayLabel(first)}</text> : null}
      {last && first !== last ? <text x={box.width - box.right} y={box.height - 8} textAnchor="end" className="fill-slate-400 text-xs">{senseDayLabel(last)}</text> : null}
    </svg>
    <figcaption className="flex flex-wrap gap-x-3 gap-y-2 text-xs">{series.map((item) => <span key={item.event} className="flex items-center gap-1.5"><span className="inline-block h-0.5 w-4" style={{ backgroundColor: item.color }} aria-hidden />{item.event_label}</span>)}</figcaption>
    {records.some((r) => r.wind_aided) ? <p className="text-xs text-slate-300">白抜きの点は追い風参考です。</p> : null}
  </figure>;
}

export function RaceRecordRow({
  record,
  kind,
  token,
  onChanged,
}: {
  record: RaceRecord;
  kind: RaceMarkKind;
  token: string | null;
  onChanged: (records: RaceRecords, event: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const lock = useActionLock();
  const confirmDialog = useConfirm();

  async function remove() {
    if (!(await confirmDialog(deleteConfirmOptions("この記録")))) return;
    if (!lock.acquire()) return;
    try {
      const res = await apiFetch<RaceRecords>(`/v1/me/race-records/${record.id}`, { method: "DELETE", token });
      onChanged(res, record.event);
    } catch (err) {
      alert(apiErrorText(err, "削除できませんでした"));
    } finally {
      lock.release();
    }
  }

  if (editing) {
    return (
      <div className="rounded-xl border border-white/20 glass bg-white/10 p-3">
        <RaceRecordForm
          token={token}
          record={record}
          defaultEvent={record.event}
          onSaved={(res, event) => {
            setEditing(false);
            onChanged(res, event);
          }}
          onCancel={() => setEditing(false)}
        />
      </div>
    );
  }

  return (
    <div className="flex items-start gap-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
      <div className="min-w-0 flex-1 space-y-1">
        <p className="text-xs text-slate-300">
          {senseDayLabel(record.raced_on)}
          {record.meet_name ? ` ・ ${record.meet_name}` : ""}
          {record.round ? ` ・ ${record.round}` : ""}
        </p>
        <p className="flex flex-wrap items-baseline gap-x-2">
          <span className="text-xs text-sky-200">{record.event_label}</span>
          <span className="text-lg font-bold tabular-nums text-slate-100">{formatRaceMark(record.mark, kind)}</span>
          {record.wind !== null ? <span className="text-xs tabular-nums text-slate-300">風 {formatRaceWind(record.wind)}</span> : null}
          {record.is_pb ? <span className="rounded-full bg-amber-300/20 px-2 py-0.5 text-xs font-semibold text-amber-200">自己ベスト</span> : null}
          {record.wind_aided ? <span className="rounded-full bg-white/10 px-2 py-0.5 text-xs text-slate-200">追い風参考</span> : null}
        </p>
        {record.memo ? <p className="whitespace-pre-wrap break-words text-sm text-slate-200">{record.memo}</p> : null}
      </div>
      <div className="flex shrink-0 gap-1">
        <button type="button" aria-label="記録を直す" onClick={() => setEditing(true)} className="rounded-md p-1.5 text-slate-300 hover:bg-white/10">
          <Pencil className="size-4" aria-hidden />
        </button>
        <button type="button" aria-label="記録を削除" onClick={remove} disabled={lock.pending} className="rounded-md p-1.5 text-red-300 hover:bg-red-950/40 disabled:opacity-50">
          <Trash2 className="size-4" aria-hidden />
        </button>
      </div>
    </div>
  );
}

type PBStats = { days: number; min_authors: number; events: { event: string; event_label: string; authors: number }[] };

// みんなの自己ベスト更新。直近30日に種目ごとに更新した人の数で、5人未満の種目は API が返さない。
function PBStatsCard({ token }: { token: string | null }) {
  const [stats, setStats] = useState<PBStats | null>(null);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<PBStats>("/v1/race-records/pb-stats", { token, signal: controller.signal })
      .then(setStats)
      .catch(() => {
        // おまけの表示なので、読めなければ出さない。
      });
    return () => controller.abort();
  }, [token]);

  if (!stats || stats.events.length === 0) return null;
  return (
    <section className="rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="text-sm font-semibold text-slate-100">この{stats.days}日で自己ベストを更新した人</h2>
      <ul className="mt-2 flex flex-wrap gap-2">
        {stats.events.map((item) => (
          <li key={item.event} className="rounded-full border border-amber-300/40 bg-amber-400/10 px-3 py-1 text-sm text-amber-100">
            {item.event_label} <span className="font-semibold">{item.authors}人</span>
          </li>
        ))}
      </ul>
      <p className="mt-2 text-xs text-slate-300">記録を残した人の数です。だれが更新したかは出しません（{stats.min_authors}人に満たない種目は出しません）。</p>
    </section>
  );
}

const noRecords: RaceRecords = { events: [], records: [] };

// 記録のタブ。書く欄、種目の切り替え、グラフ、記録の一覧(新しい順)。
export function RaceRecordsPanel({ token, data: externalData, onRecordsChanged, selectedDay }: {
  token: string | null;
  data?: RaceRecords | null;
  onRecordsChanged?: (records: RaceRecords) => void;
  selectedDay?: string | null;
}) {
  const [data, setData] = useState<RaceRecords | null>(null);
  const [failed, setFailed] = useState(false);
  const [selected, setSelected] = useState<string>("");

  useEffect(() => {
    if (!token || externalData !== undefined) return;
    const controller = new AbortController();
    apiFetch<RaceRecords>("/v1/me/race-records", { token, signal: controller.signal })
      .then(setData)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [token, externalData]);

  if (failed) return <ErrorState message="記録の取得に失敗しました。APIの起動を確認してください。" />;
  // ログインしていない人には、空の記録の画面を見せる(書こうとするとログインへ案内する)。
  const shown = token ? (externalData === undefined ? data : externalData) : noRecords;
  if (!shown) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const events = sortRaceEvents(shown.events);
  const current = events.find((item) => item.event === selected) ?? events[0];
  const overview = selected === "all";
  const records = sortRaceRecords(overview ? shown.records : current ? shown.records.filter((record) => record.event === current.event) : []);

  function onSaved(next: RaceRecords, event: string) {
    setData(next);
    onRecordsChanged?.(next);
    setSelected((previous) => previous === "all" ? "all" : event);
  }

  return (
    <div className="space-y-4">
      <PBStatsCard token={token} />
      <section className="rounded-2xl border border-white/20 glass bg-white/10 p-4">
        <RaceRecordForm key={selectedDay ?? "today"} defaultDay={selectedDay ?? undefined} token={token} defaultEvent={current?.event ?? raceRecordEvents[0].value} onSaved={onSaved} />
      </section>

      {!current ? (
        <EmptyState title="まだ記録がありません" body="競技会の記録を残すと、種目ごとの推移がグラフになります。" />
      ) : (
        <section className="space-y-3">
          <div className="flex gap-2 overflow-x-auto pb-1" aria-label="グラフの種目">
            <button type="button" onClick={() => setSelected("all")} aria-pressed={overview} className={`shrink-0 rounded-full border px-3 py-1 text-sm ${overview ? "border-sky-300 bg-sky-400/20 text-sky-100" : "border-white/20 bg-white/10 text-slate-200"}`}>全種目<span className="ml-1 text-xs">{shown.records.length}</span></button>
            {events.map((item) => (
              <button
                key={item.event}
                type="button"
                onClick={() => setSelected(item.event)}
                aria-pressed={!overview && item.event === current.event}
                className={`shrink-0 rounded-full border px-3 py-1 text-sm ${
                  !overview && item.event === current.event ? "border-sky-300 bg-sky-400/20 text-sky-100" : "border-white/20 bg-white/10 text-slate-200"
                }`}
              >
                {item.event_label}
                <span className="ml-1 text-xs text-slate-300">{item.count}</span>
              </button>
            ))}
          </div>
          <div className="rounded-2xl border border-white/20 glass bg-white/10 p-3">
            {overview ? <RaceOverviewChart records={records} /> : <RaceRecordChart records={records} kind={current.kind} />}
            {!overview ? <p className="mt-2 text-xs text-slate-400">小さな変化を大きく見せないよう、縦軸に余裕を持たせています。同日の記録はラウンド順に並びます。</p> : null}
          </div>
          <ul className="space-y-2">
            {[...records].reverse().map((record) => (
              <li key={record.id}>
                <RaceRecordRow record={record} kind={raceMarkKind(record.event)} token={token} onChanged={onSaved} />
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
