"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import {
  coachPledgeItems,
  highPlanPriceYen,
  maxTrialPriceYen,
  minPlanPriceYen,
  planDurationLabel,
  firstVideoWaitDays,
  planDurations,
  planIncludes,
  planKindLabels,
  planMenuUpdatesLabels,
  planReplyFormatLabels,
  planSummary,
  planVideoReviewChoices,
  sortTrialFirst,
  videoReviewsLabel,
} from "@/lib/coaching-plan";
import { formatYen } from "@/lib/format";
import type {
  CoachApplication,
  CoachingPlan,
  CoachingPlanKind,
  PlanIncludes,
  PlanMenuUpdates,
  PlanReplyFormat,
} from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useEffect, useState } from "react";

// パーソナル指導のプラン。コーチ画面では誓約と、プランの作成・編集をする。コーチ一覧では公開中のプランを見せる。

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

function CoachPledgeForm({ token, onSaved }: { token: string | null; onSaved: (c: CoachApplication) => void }) {
  const [agree, setAgree] = useState(false);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function submit() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me/pledge", {
        method: "POST",
        token,
        body: JSON.stringify({ agree }),
      });
      onSaved(d.application);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.agree || err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-slate-200">プランを作る前に、次のことを約束してください。利用者の多くは中高生です。</p>
      <ul className="list-disc space-y-1.5 pl-5 text-sm text-slate-100">
        {coachPledgeItems.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
      <label className="flex items-start gap-2 text-sm text-white">
        <input type="checkbox" className="mt-1 size-4" checked={agree} onChange={(e) => setAgree(e.target.checked)} />
        <span>上のことを守ります</span>
      </label>
      <Button type="button" className="w-full" disabled={!agree || pending} onClick={() => void submit()}>
        誓約する
      </Button>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

type PlanDraft = PlanIncludes & {
  kind: CoachingPlanKind;
  duration_days: number;
  title: string;
  description: string;
  price: string;
  capacity: string;
};

function draftOf(plan: CoachingPlan | null): PlanDraft {
  if (!plan) {
    return {
      kind: "period",
      duration_days: 30,
      title: "",
      description: "",
      price: "",
      capacity: "3",
      video_reviews: null,
      menu_updates: null,
      reply_formats: [],
      start_on_first_video: false,
    };
  }
  return {
    kind: plan.kind,
    duration_days: plan.duration_days,
    title: plan.title,
    description: plan.description,
    price: String(plan.price_yen),
    capacity: String(plan.capacity),
    video_reviews: plan.video_reviews,
    menu_updates: plan.menu_updates,
    reply_formats: plan.reply_formats,
    start_on_first_video: plan.start_on_first_video,
  };
}

// PlanIncludesList は、プランで「何が返ってくるか」。書いていない項目は出さない。
export function PlanIncludesList({ plan }: { plan: PlanIncludes }) {
  const rows = planIncludes(plan);
  if (rows.length === 0) return null;
  return (
    <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
      {rows.map((r) => (
        <div key={r.label} className="contents">
          <dt className="text-slate-300">{r.label}</dt>
          <dd className="text-slate-100">{r.value}</dd>
        </div>
      ))}
    </dl>
  );
}

// PlanIncludesFields は、プランの「何が返ってくるか」を選ぶ欄。どれも「書かない」を選べる。
function PlanIncludesFields({
  draft,
  setDraft,
  fields,
}: {
  draft: PlanDraft;
  setDraft: (f: (d: PlanDraft) => PlanDraft) => void;
  fields: Record<string, string>;
}) {
  // 前に別の回数を保存していたら、その回数も選べるように並べる。
  const reviewChoices =
    draft.video_reviews === null || planVideoReviewChoices.includes(draft.video_reviews)
      ? planVideoReviewChoices
      : [draft.video_reviews, ...planVideoReviewChoices];
  const fieldError = (key: string) => (fields[key] ? <p className="text-sm text-red-400">{fields[key]}</p> : null);
  const toggleFormat = (f: PlanReplyFormat) =>
    setDraft((d) => ({
      ...d,
      reply_formats: d.reply_formats.includes(f) ? d.reply_formats.filter((x) => x !== f) : [...d.reply_formats, f],
    }));

  return (
    <div className="space-y-3 rounded-lg border border-white/15 p-3">
      <div className="space-y-0.5">
        <p className="text-sm font-semibold text-white">何が返ってくるか</p>
        <p className="text-xs text-slate-300">書いておくと、コーチ一覧で利用者が比べやすくなります。書かなくても保存できます。</p>
      </div>
      <div className="space-y-1.5">
        <p className="text-sm text-slate-200">動画への返答</p>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            aria-pressed={draft.video_reviews === null}
            onClick={() => setDraft((d) => ({ ...d, video_reviews: null }))}
            className={chip(draft.video_reviews === null)}
          >
            書かない
          </button>
          {reviewChoices.map((n) => (
            <button
              key={n}
              type="button"
              aria-pressed={draft.video_reviews === n}
              onClick={() => setDraft((d) => ({ ...d, video_reviews: n }))}
              className={chip(draft.video_reviews === n)}
            >
              {videoReviewsLabel(n)}
            </button>
          ))}
        </div>
        {fieldError("video_reviews")}
      </div>
      <div className="space-y-1.5">
        <p className="text-sm text-slate-200">練習メニュー</p>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            aria-pressed={draft.menu_updates === null}
            onClick={() => setDraft((d) => ({ ...d, menu_updates: null }))}
            className={chip(draft.menu_updates === null)}
          >
            書かない
          </button>
          {(Object.keys(planMenuUpdatesLabels) as PlanMenuUpdates[]).map((m) => (
            <button
              key={m}
              type="button"
              aria-pressed={draft.menu_updates === m}
              onClick={() => setDraft((d) => ({ ...d, menu_updates: m }))}
              className={chip(draft.menu_updates === m)}
            >
              {planMenuUpdatesLabels[m]}
            </button>
          ))}
        </div>
        {fieldError("menu_updates")}
      </div>
      <div className="space-y-1.5">
        <p className="text-sm text-slate-200">返し方（いくつでも）</p>
        <div className="flex flex-wrap gap-2">
          {(Object.keys(planReplyFormatLabels) as PlanReplyFormat[]).map((f) => (
            <button
              key={f}
              type="button"
              aria-pressed={draft.reply_formats.includes(f)}
              onClick={() => toggleFormat(f)}
              className={chip(draft.reply_formats.includes(f))}
            >
              {planReplyFormatLabels[f]}
            </button>
          ))}
        </div>
        {fieldError("reply_formats")}
      </div>
      <label className="flex items-start gap-2 text-sm text-slate-200">
        <input
          type="checkbox"
          className="mt-1 size-4"
          checked={draft.start_on_first_video}
          onChange={(e) => setDraft((d) => ({ ...d, start_on_first_video: e.target.checked }))}
        />
        <span>
          期間を、最初の動画が届いた日から数える
          <span className="block text-xs text-slate-300">
            払ったのに撮影できず日数が減る、を防げます。{firstVideoWaitDays}日のあいだに届かなければ、その時から数え始めます。
          </span>
        </span>
      </label>
    </div>
  );
}

function PlanForm({
  plan,
  token,
  onSaved,
  onCancel,
}: {
  plan: CoachingPlan | null;
  token: string | null;
  onSaved: (p: CoachingPlan) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState(() => draftOf(plan));
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  function setKind(kind: CoachingPlanKind) {
    setDraft((d) => ({ ...d, kind, duration_days: planDurations[kind].includes(d.duration_days) ? d.duration_days : planDurations[kind][0] }));
  }

  async function submit() {
    const price = Number(draft.price.replace(/[,，円\s]/g, ""));
    const capacity = Number(draft.capacity);
    let confirmHigh = false;
    if (Number.isFinite(price) && price > highPlanPriceYen && price !== plan?.price_yen) {
      if (!confirm(`${formatYen(price)}で保存します。打ち間違いではありませんか？`)) return;
      confirmHigh = true;
    }
    if (!acquire()) return;
    setError("");
    setFields({});
    const body = {
      kind: draft.kind,
      duration_days: draft.duration_days,
      title: draft.title,
      description: draft.description,
      price_yen: Number.isFinite(price) ? Math.trunc(price) : null,
      capacity: Number.isFinite(capacity) ? Math.trunc(capacity) : null,
      published: plan ? plan.published : true,
      confirm_high_price: confirmHigh,
      video_reviews: draft.video_reviews,
      menu_updates: draft.menu_updates,
      reply_formats: draft.reply_formats,
      start_on_first_video: draft.start_on_first_video,
    };
    try {
      const d = await apiFetch<{ plan: CoachingPlan }>(plan ? `/v1/coach/me/plans/${plan.id}` : "/v1/coach/me/plans", {
        method: plan ? "PATCH" : "POST",
        token,
        body: JSON.stringify(body),
      });
      onSaved(d.plan);
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields) setFields(err.fields);
      else setError(err instanceof ApiRequestError ? err.message : "保存できませんでした");
    } finally {
      release();
    }
  }

  const fieldError = (key: string) => (fields[key] ? <p className="text-sm text-red-400">{fields[key]}</p> : null);

  return (
    <div className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-3">
      <div className="space-y-1.5">
        <p className="text-sm text-slate-200">種類</p>
        <div className="flex flex-wrap gap-2">
          {(Object.keys(planKindLabels) as CoachingPlanKind[]).map((k) => (
            <button key={k} type="button" aria-pressed={draft.kind === k} onClick={() => setKind(k)} className={chip(draft.kind === k)}>
              {planKindLabels[k]}
            </button>
          ))}
        </div>
        <p className="text-xs text-slate-300">
          {draft.kind === "period"
            ? "期間中、メニュー作りや動画への返答を続けて見ます。"
            : draft.kind === "single"
              ? "動画を詳しく見て返すなど、7日以内に終わる1回きりの指導です。"
              : `あなたの指導をはじめて受ける人だけが申し込める、7日以内の安いお試しです（${formatYen(minPlanPriceYen)}〜${formatYen(maxTrialPriceYen)}、1つまで）。コーチ一覧ではいちばん上に出ます。`}
        </p>
        {fieldError("kind")}
      </div>
      {planDurations[draft.kind].length > 1 ? (
        <div className="space-y-1.5">
          <p className="text-sm text-slate-200">期間</p>
          <div className="flex flex-wrap gap-2">
            {planDurations[draft.kind].map((days) => (
              <button
                key={days}
                type="button"
                aria-pressed={draft.duration_days === days}
                onClick={() => setDraft((d) => ({ ...d, duration_days: days }))}
                className={chip(draft.duration_days === days)}
              >
                {planDurationLabel(draft.kind, days)}
              </button>
            ))}
          </div>
          {fieldError("duration_days")}
        </div>
      ) : null}
      <label className="block space-y-1.5 text-sm text-slate-200">
        <span>プラン名（60文字まで）</span>
        <Input value={draft.title} maxLength={60} onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))} placeholder="例: 1か月で100mのスタートを直す" />
        {fieldError("title")}
      </label>
      <label className="block space-y-1.5 text-sm text-slate-200">
        <span>内容（1000文字まで）</span>
        <Textarea
          rows={5}
          value={draft.description}
          maxLength={1000}
          onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))}
          placeholder="例: 最初に動画を見て、1か月分の練習メニューを作ります。週に1回、送ってもらった動画に返答します。"
        />
        {fieldError("description")}
      </label>
      <label className="block space-y-1.5 text-sm text-slate-200">
        <span>
          値段（{formatYen(minPlanPriceYen)}
          {draft.kind === "trial" ? `〜${formatYen(maxTrialPriceYen)}` : "から"}）
        </span>
        <Input
          inputMode="numeric"
          value={draft.price}
          onChange={(e) => setDraft((d) => ({ ...d, price: e.target.value }))}
          placeholder={draft.kind === "trial" ? "2500" : "8000"}
        />
        <p className="text-xs text-slate-300">TrackTalk の手数料（20%）を引いた分が報酬になります。</p>
        {fieldError("price_yen")}
      </label>
      <PlanIncludesFields draft={draft} setDraft={setDraft} fields={fields} />
      <label className="block space-y-1.5 text-sm text-slate-200">
        <span>同時に受ける人数（1〜50人）</span>
        <Input inputMode="numeric" value={draft.capacity} onChange={(e) => setDraft((d) => ({ ...d, capacity: e.target.value }))} />
        {fieldError("capacity")}
      </label>
      <div className="flex gap-2">
        <Button type="button" className="flex-1" disabled={pending} onClick={() => void submit()}>
          保存する
        </Button>
        <button
          type="button"
          disabled={pending}
          onClick={onCancel}
          className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
        >
          やめる
        </button>
      </div>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

// CoachPlansSection は、コーチ画面の「パーソナル指導のプラン」。誓約がまだなら誓約から。
export function CoachPlansSection({
  coach,
  token,
  onCoachChange,
}: {
  coach: CoachApplication;
  token: string | null;
  onCoachChange: (c: CoachApplication) => void;
}) {
  const [plans, setPlans] = useState<CoachingPlan[] | null>(null);
  const [editing, setEditing] = useState<string | "new" | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ plans: CoachingPlan[] }>("/v1/coach/me/plans", { token })
      .then((d) => {
        if (!cancelled) setPlans(d.plans);
      })
      .catch(() => {
        if (!cancelled) setError("プランを読み込めませんでした");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  function saved(p: CoachingPlan) {
    setPlans((list) => {
      const current = list ?? [];
      return current.some((x) => x.id === p.id) ? current.map((x) => (x.id === p.id ? p : x)) : [...current, p];
    });
    setEditing(null);
  }

  async function togglePublished(p: CoachingPlan) {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ plan: CoachingPlan }>(`/v1/coach/me/plans/${p.id}`, {
        method: "PATCH",
        token,
        body: JSON.stringify({
          kind: p.kind,
          duration_days: p.duration_days,
          title: p.title,
          description: p.description,
          price_yen: p.price_yen,
          capacity: p.capacity,
          published: !p.published,
          video_reviews: p.video_reviews,
          menu_updates: p.menu_updates,
          reply_formats: p.reply_formats,
          start_on_first_video: p.start_on_first_video,
        }),
      });
      saved(d.plan);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "保存できませんでした");
    } finally {
      release();
    }
  }

  return (
    <section className="space-y-3">
      <div className="space-y-1">
        <h2 className="font-bold">パーソナル指導のプラン</h2>
        <p className="text-xs text-slate-300">公開中のプランは、コーチ一覧に出て、申し込みを受け付けます。</p>
      </div>
      {!coach.pledged ? (
        <div className="rounded-xl border border-white/20 glass bg-white/10 p-3">
          <CoachPledgeForm token={token} onSaved={onCoachChange} />
        </div>
      ) : (
        <>
          {plans === null && !error ? <p className="text-sm text-slate-200">読み込み中...</p> : null}
          {plans?.length === 0 && editing !== "new" ? <p className="text-sm text-slate-300">まだプランがありません</p> : null}
          <ul className="space-y-2">
            {plans?.map((p) =>
              editing === p.id ? (
                <li key={p.id}>
                  <PlanForm plan={p} token={token} onSaved={saved} onCancel={() => setEditing(null)} />
                </li>
              ) : (
                <li key={p.id} className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="font-semibold text-white">{p.title}</p>
                      <p className="text-xs text-slate-300">
                        {planSummary(p)} · {p.capacity}人まで
                      </p>
                      <PlanIncludesList plan={p} />
                    </div>
                    <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs ${p.published ? "bg-blue-500/30 text-white" : "bg-white/15 text-slate-300"}`}>
                      {p.published ? "公開中" : "非公開"}
                    </span>
                  </div>
                  <p className="whitespace-pre-wrap text-slate-200">{p.description}</p>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      disabled={pending || editing !== null}
                      onClick={() => setEditing(p.id)}
                      className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
                    >
                      直す
                    </button>
                    <button
                      type="button"
                      disabled={pending || editing !== null}
                      onClick={() => void togglePublished(p)}
                      className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
                    >
                      {p.published ? "非公開にする" : "公開する"}
                    </button>
                  </div>
                </li>
              ),
            )}
          </ul>
          {editing === "new" ? (
            <PlanForm plan={null} token={token} onSaved={saved} onCancel={() => setEditing(null)} />
          ) : plans && plans.length < 10 ? (
            <Button type="button" className="w-full" disabled={editing !== null} onClick={() => setEditing("new")}>
              プランを追加する
            </Button>
          ) : null}
        </>
      )}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}

// TrialBadge は、お試しプランにつける印。
export function TrialBadge() {
  return (
    <p className="mb-1 inline-flex rounded-full bg-blue-500/30 px-2 py-0.5 text-xs font-semibold text-blue-100">
      はじめての人だけのお試し
    </p>
  );
}

// PublicPlanList は、コーチ一覧のカードに出すプラン。
export function PublicPlanList({ plans }: { plans: CoachingPlan[] }) {
  if (plans.length === 0) return null;
  return (
    <div className="space-y-1.5 border-t border-white/15 pt-2">
      <p className="text-xs text-slate-300">パーソナル指導</p>
      <ul className="space-y-2">
        {sortTrialFirst(plans).map((p) => (
          <li key={p.id} className={`rounded-lg p-2 ${p.kind === "trial" ? "border border-blue-400/40 bg-blue-500/10" : "bg-white/5"}`}>
            {p.kind === "trial" ? <TrialBadge /> : null}
            <p className="font-semibold text-white">{p.title}</p>
            <p className="text-xs text-slate-300">{planSummary(p)}</p>
            <PlanIncludesList plan={p} />
            <p className="mt-1 whitespace-pre-wrap text-slate-200">{p.description}</p>
            {p.full ? (
              <p className="mt-2 text-xs font-semibold text-amber-300">いまは満員です</p>
            ) : (
              <Link
                href={`/coaching/personal/plans/${p.id}`}
                className="mt-2 flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white"
              >
                このプランに申し込む
              </Link>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
