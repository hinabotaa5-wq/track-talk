"use client";
import { ReferralInput, checkReferralInput, type ReferralWarning } from "@/components/coaching-referral";
import { L } from "@/lib/i18n";

import { useSolicitationGuard } from "@/components/contact-notice";
import { PageLoading } from "@/components/page-loading";
import { useAuth } from "@/components/auth-provider";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { PurchaseConfirm } from "@/components/purchase-confirm";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, apiErrorText, ApiRequestError } from "@/lib/api";
import { loginPathFrom } from "@/lib/safe-next";
import { purchaseNoticeShown, useMinorDeclaration } from "@/lib/minor-purchase";
import { coachEventLabels } from "@/lib/coach-recruit";
import { eagerVideoQuery, uploadVideoToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { maxReviewSeconds, reviewEvents, reviewPayMethodLabels, tooLong, videoDuration } from "@/lib/review";
import { couponCashPrice, couponCount, type TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime, formatDisplayPrice } from "@/lib/format";
import { defaultGroupTitle, maxPremiumVideos, premiumGroupInputLimits, premiumKindLabels, premiumKinds, premiumVideoInputLimits } from "@/lib/premium";
import { premiumConfirmRows } from "@/lib/premium-confirm";
import type { CoachEvent, PremiumKind, ReviewCoachOption, ReviewEstimate, ReviewRequest } from "@/lib/types";
import { looksLikeContact } from "@/lib/solicitation";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

// プレミアムの申込（本体 #1053、補足 7231984993・補足2 7231991592）。/coaching/new?plan=premium&coach=… で開く（コーチ一覧のプレミアムタブから）。
// 動画をテーマごとのグループに分けて、合わせて1〜6本を送る。グループはテンプレ（レース全体／スタート／後半／練習／その他）から作り、
// テーマ名は変えられる。グループ数・グループごとの本数に制限はない（練習の動画だけ・1グループに6本でもよい）。
// 入力はすべて任意：PB・目標タイムは申込全体に1回、グループごとに「何を意識して取り組んでいるか・何が目的か」、
// レース全体のグループの動画ごとに「この時のタイム・風速・その時の体のコンディション」。相談内容の自由記述は置かない。
// 価格はコーチのプレミアム価格（API の有効価格）。支払いはカード（クーポン併用可）。チケットは使えない。
// 未成年の宣言・紹介ID・最終確認・カードの仮押さえはスタンダードと同じ流れ。

const maxBytes = 100 * 1024 * 1024;

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

type DraftVideo = {
  key: number;
  file: File;
  race_time: string;
  wind_speed: string;
  body_condition: string;
};

type DraftGroup = {
  key: number;
  kind: PremiumKind;
  title: string;
  focus_note: string;
  videos: DraftVideo[];
};

type VideoTextKey = "race_time" | "wind_speed" | "body_condition";

// レース全体のグループの動画にだけ出す入力。
function raceInputs(): { key: VideoTextKey; label: string; placeholder: string; multiline?: boolean }[] {
  return [
    { key: "race_time", label: L("この時のタイム"), placeholder: L("例：11秒02") },
    { key: "wind_speed", label: L("風速"), placeholder: L("例：+1.2、向かい風で強い") },
    { key: "body_condition", label: L("その時の体のコンディション"), placeholder: L("例：前日に脚が張っていた"), multiline: true },
  ];
}

// 送る形：グループの意識・目的と、レース全体のグループの動画の入力だけを送る。空白だけ・未入力は null（サーバーも NULL で保存する）。
function groupBody(group: DraftGroup, publicIDs: string[]) {
  const text = (v: string) => v.trim() || null;
  return {
    kind: group.kind,
    title: group.title.trim() || null,
    focus_note: text(group.focus_note),
    videos: group.videos.map((video, i) => ({
      video_public_id: publicIDs[i],
      ...(group.kind === "race" ? { race_time: text(video.race_time), wind_speed: text(video.wind_speed), body_condition: text(video.body_condition) } : {}),
    })),
  };
}

export function PremiumRequestForm() {
  const { session, token, loading, profile } = useAuth();
  const router = useRouter();
  const tier = "coach";
  const [requestedCoach, setRequestedCoach] = useState<string | null>(null);
  const [event, setEvent] = useState<CoachEvent>(reviewEvents[0]);
  const [coach, setCoach] = useState<ReviewCoachOption | null>(null);
  const [coachChecked, setCoachChecked] = useState(false);
  const [estimate, setEstimate] = useState<ReviewEstimate | null>(null);
  const [ticketSummary, setTicketSummary] = useState<TicketSummary | null>(null);
  const [pb, setPb] = useState("");
  const [goalTime, setGoalTime] = useState("");
  const [groups, setGroups] = useState<DraftGroup[]>([]);
  const nextKey = useRef(1);
  const videos = groups.flatMap((group) => group.videos);
  const [method, setMethod] = useState<"card" | "coupon">("card");
  const [referralCode, setReferralCode] = useState("");
  const [referralWarning, setReferralWarning] = useState<ReferralWarning>(false);
  const [error, setError] = useState("");
  const [contactField, setContactField] = useState<string | null>(null);
  const guard = useSolicitationGuard();
  const declaration = useMinorDeclaration();
  const declared = declaration.checked;
  const [confirming, setConfirming] = useState(false);
  const [upload, setUpload] = useState<{ index: number; progress: CloudinaryUploadProgress; file: File } | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const { pending, acquire, release } = useActionLock();

  const priceYen = coach?.premium_price_yen ?? 0;
  const couponsLeft = couponCount(ticketSummary);
  const couponsUsable = couponsLeft >= 1;
  const couponYen = method === "coupon" ? Math.min(ticketSummary?.coupons?.amount_yen ?? 500, priceYen) : 0;
  const cashPrice = couponCashPrice(priceYen, couponYen);
  const cashPayment = !!estimate?.payments && cashPrice > 0;
  const needsDeclaration = cashPayment && purchaseNoticeShown(profile);
  const referralEligible = cashPayment && method !== "coupon";

  useEffect(() => {
    if (!loading && !session) router.replace(loginPathFrom(window.location));
  }, [loading, session, router]);

  useEffect(() => {
    // 静的書き出しのため、URL のクエリは表示後にしか読めない。
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRequestedCoach(new URLSearchParams(window.location.search).get("coach")?.trim() || "");
  }, []);

  // 指定のコーチがプレミアムを受け付けている種目を探す。担当種目の並びで最初に見つかった種目を初期値にする。
  useEffect(() => {
    if (!token || requestedCoach === null) return;
    if (!requestedCoach) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setCoachChecked(true);
      return;
    }
    let cancelled = false;
    void (async () => {
      const lists = await Promise.all(reviewEvents.map(async (reviewEvent) => {
        try {
          const data = await apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(reviewEvent)}&plan=premium`, { token });
          return { event: reviewEvent, coach: data.coaches.find((c) => c.id === requestedCoach) ?? null };
        } catch {
          return { event: reviewEvent, coach: null };
        }
      }));
      if (cancelled) return;
      const accepting = lists.filter((item) => item.coach);
      const preferred = accepting.find((item) => item.coach?.events[0] === item.event) ?? accepting[0];
      if (preferred) {
        setEvent(preferred.event);
        setCoach(preferred.coach);
      }
      setCoachChecked(true);
    })();
    return () => { cancelled = true; };
  }, [token, requestedCoach]);

  // 種目を変えたら、その種目でもプレミアムを受け付けているかを確かめ直す（価格もそのとき取り直す）。
  async function pickEvent(next: CoachEvent) {
    if (next === event || !requestedCoach) return;
    setEvent(next);
    try {
      const data = await apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(next)}&plan=premium`, { token });
      setCoach(data.coaches.find((c) => c.id === requestedCoach) ?? null);
    } catch {
      setCoach(null);
    }
  }

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<ReviewEstimate>(`/v1/coaching/estimate?tier=${tier}&event=${encodeURIComponent(event)}`, { token })
      .then((d) => { if (!cancelled) setEstimate(d); })
      .catch(() => { if (!cancelled) setEstimate(null); });
    return () => { cancelled = true; };
  }, [token, event]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token })
      .then((data) => { if (!cancelled) setTicketSummary(data); })
      .catch(() => { if (!cancelled) setTicketSummary(null); });
    return () => { cancelled = true; };
  }, [token]);

  function addGroup(kind: PremiumKind) {
    setError("");
    const key = nextKey.current++;
    setGroups((prev) => [...prev, { key, kind, title: defaultGroupTitle(kind, prev.map((g) => g.title.trim())), focus_note: "", videos: [] }]);
  }

  function updateGroup(key: number, patch: Partial<DraftGroup>) {
    setGroups((prev) => prev.map((g) => (g.key === key ? { ...g, ...patch } : g)));
    setContactField(null);
  }

  // テンプレを変えたとき、テーマ名がまだ前のテンプレの既定のままなら、新しいテンプレの既定に合わせる。
  function changeKind(group: DraftGroup, kind: PremiumKind) {
    const others = groups.filter((g) => g.key !== group.key).map((g) => g.title.trim());
    const untouched = group.title.trim() === "" || group.title.trim() === defaultGroupTitle(group.kind, others);
    updateGroup(group.key, { kind, ...(untouched ? { title: defaultGroupTitle(kind, others) } : {}) });
  }

  function updateVideo(groupKey: number, videoKey: number, patch: Partial<DraftVideo>) {
    setGroups((prev) => prev.map((g) => (g.key === groupKey ? { ...g, videos: g.videos.map((v) => (v.key === videoKey ? { ...v, ...patch } : v)) } : g)));
    setContactField(null);
  }

  async function addVideo(groupKey: number, picked: File | null) {
    setError("");
    if (!picked) return;
    // 種別（MIME）が動画でないファイルは、容量・長さを見る前に知らせる（本体 #1109）。
    if (!picked.type.startsWith("video/")) {
      setError(L("動画ファイルを選んでください"));
      return;
    }
    if (videos.length >= maxPremiumVideos) {
      setError(L("動画は全部で{max}本までです", { max: maxPremiumVideos }));
      return;
    }
    if (picked.size > maxBytes) {
      setError(L("100MB以内の動画にしてください"));
      return;
    }
    if (tooLong(await videoDuration(picked))) {
      setError(L("動画は{maxReviewSeconds}秒以内にしてください", { maxReviewSeconds }));
      return;
    }
    const key = nextKey.current++;
    setGroups((prev) => prev.flatMap((g) => g.videos).length >= maxPremiumVideos ? prev
      : prev.map((g) => (g.key === groupKey ? { ...g, videos: [...g.videos, { key, file: picked, race_time: "", wind_speed: "", body_condition: "" }] } : g)));
  }

  function texts(): { field: string; value: string }[] {
    const list = [{ field: "pb", value: pb }, { field: "goal_time", value: goalTime }];
    for (const group of groups) {
      list.push({ field: `${group.key}.title`, value: group.title }, { field: `${group.key}.focus_note`, value: group.focus_note });
      if (group.kind !== "race") continue;
      for (const video of group.videos) {
        for (const input of raceInputs()) list.push({ field: `${group.key}.${video.key}.${input.key}`, value: video[input.key] });
      }
    }
    return list;
  }

  function validate(): boolean {
    if (!estimate) { setError(L("料金と受付状況を確認してから送信してください。")); return false; }
    if (!coach) { setError(L("このコーチは現在、プレミアムを受け付けていません")); return false; }
    if (!guard.allow(texts().map((t) => t.value).join("\n"))) return false;
    if (videos.length === 0) { setError(L("動画を1本以上追加してください")); return false; }
    if (videos.length > maxPremiumVideos) { setError(L("動画は全部で{max}本までです", { max: maxPremiumVideos })); return false; }
    if (groups.some((g) => g.videos.length === 0)) { setError(L("動画が入っていないグループがあります。動画を入れるか、グループを外してください。")); return false; }
    const titles = groups.map((g) => g.title.trim() || premiumKindLabels[g.kind]);
    if (new Set(titles).size !== titles.length) { setError(L("同じテーマ名のグループがあります。テーマ名を変えてください")); return false; }
    if (method === "coupon" && !couponsUsable) { setError(L("使えるクーポンがありません。別のお支払い方法を選んでください。")); return false; }
    if (needsDeclaration && !declared) { setError(L("購入前の注意事項を確認して、チェックを入れてください。")); return false; }
    setError("");
    return true;
  }

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const contact = texts().find((t) => looksLikeContact(t.value));
    if (contact) {
      setError("");
      setContactField(contact.field);
      return;
    }
    if (!validate()) return;
    setConfirming(true);
  }

  async function send() {
    if (!coach || !validate() || !acquire()) return;
    setError("");
    try {
      const referralCheck = referralEligible ? await checkReferralInput(referralCode, token) : "ok";
      if (referralCheck !== "ok") {
        setReferralCode("");
        setReferralWarning(referralCheck);
        return;
      }
      // 動画はグループの順・グループ内の追加した順に1本ずつ上げる。
      const uploaded: ReturnType<typeof groupBody>[] = [];
      setUploadPhase("upload");
      let index = 0;
      for (const group of groups) {
        const ids: string[] = [];
        for (const video of group.videos) {
          const at = index++;
          const signed = await apiFetch<CloudinarySignature>(`/v1/uploads/coaching-video${eagerVideoQuery}`, { method: "POST", token });
          setUpload({ index: at, file: video.file, progress: { loaded: 0, total: video.file.size, percent: 0 } });
          const payload = await uploadVideoToCloudinary(video.file, signed, token, (progress) => setUpload({ index: at, file: video.file, progress }));
          if (payload.public_id !== signed.public_id) throw new Error(L("アップロードに失敗しました"));
          ids.push(payload.public_id);
        }
        uploaded.push(groupBody(group, ids));
      }
      setUploadPhase("saving");
      const created = await apiFetch<ReviewRequest>("/v1/coaching/requests", {
        method: "POST",
        token,
        body: JSON.stringify({
          plan: "premium", tier, event, pb: pb.trim() || null, goal_time: goalTime.trim() || null, groups: uploaded,
          method: "card", use_coupon: method === "coupon",
          referral_code: referralEligible ? referralCode : undefined, requested_coach_id: coach.id, ...declaration.body(needsDeclaration),
        }),
      });
      if (created.checkout_url) {
        window.location.assign(created.checkout_url);
        return;
      }
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : L("送信に失敗しました")));
      if (err instanceof ApiRequestError && err.code === "coupon_unavailable") {
        try { setTicketSummary(await apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token })); } catch { /* 元のエラーを表示する。 */ }
      }
      setUpload(null);
      setConfirming(false);
      release();
    }
  }

  if (loading || !session || !coachChecked) return <PageLoading screen label={L("読み込み中...")} />;

  const heading = (
    <div className="space-y-2">
      <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">{L("プレミアム")}</p>
      <h1 className="text-2xl font-bold">{L("プレミアムを申し込む")}</h1>
      {coach ? <p className="text-sm text-slate-200">{L("担当：{name}", { name: coach.nickname ?? L("名前なし") })}</p> : null}
    </div>
  );

  if (!coach) {
    return (
      <div className="mx-auto max-w-md space-y-4">
        {heading}
        <p role="note" className="text-sm text-amber-200">{L("このコーチは現在、プレミアムを受け付けていません")}</p>
        <Link href="/coaching?tab=premium" className="text-sm text-blue-300 underline">{L("プレミアムのコーチ一覧")}</Link>
      </div>
    );
  }

  if (confirming) {
    return (
      <div className="mx-auto max-w-md space-y-3">
        {heading}
        <PurchaseConfirm
          rows={premiumConfirmRows({
            priceYen, couponYen, eventLabel: coachEventLabels[event], coachName: coach.nickname ?? undefined, pb, goalTime,
            groups: groups.map((g) => ({ title: g.title.trim() || premiumKindLabels[g.kind], count: g.videos.length })),
          })}
          confirmLabel={method === "coupon" && !cashPayment ? L("確定してクーポンで送る") : L("確定してお支払いへ")}
          onConfirm={() => void send()}
          onBack={() => { setConfirming(false); setError(""); }}
          pending={pending}
          pendingLabel={upload ? L("動画を送っています（{n}/{total}本目）", { n: upload.index + 1, total: videos.length }) : cashPayment ? L("お支払い画面へ移動しています") : undefined}
        >
          {referralEligible ? <ReferralInput token={token} code={referralCode} warning={referralWarning} disabled={pending}
            onChange={(code) => { setReferralCode(code); setReferralWarning(false); }} /> : null}
          <Link href="/legal/commerce" className="inline-block text-xs text-slate-200 underline">{L("特定商取引法に基づく表記")}</Link>
        </PurchaseConfirm>
        {upload ? <UploadProgress noun="動画" phase={uploadPhase} progress={upload.progress} fileName={upload.file.name} fileSize={upload.file.size} /> : null}
        {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      </div>
    );
  }

  const contactError = <span role="alert" className="block text-sm text-red-400">{L("外部の連絡先（URL・メール・SNS IDなど）は送れません。")}</span>;

  return (
    <div className="mx-auto max-w-md space-y-6">
      {heading}
      <form onSubmit={onSubmit} className="space-y-5" data-testid="premium-form">
        <fieldset disabled={pending} className="space-y-2">
          <legend className="text-sm font-semibold">{L("種目")}</legend>
          <div className="flex flex-wrap gap-2">
            {reviewEvents.map((value) => (
              <button key={value} type="button" aria-pressed={event === value} onClick={() => void pickEvent(value)} className={chip(event === value)}>
                {coachEventLabels[value]}
              </button>
            ))}
          </div>
        </fieldset>

        <fieldset className="space-y-3 text-sm">
          <legend className="font-semibold">{L("選手情報（任意）")}</legend>
          {guard.warning}
          <label className="block space-y-1">
            <span className="block text-xs font-semibold text-slate-300">{L("PB（自己ベスト）")}</span>
            <Input name="pb" value={pb} onChange={(e) => { setPb(e.target.value); setContactField(null); }} placeholder={L("例：11秒20（2025年）")} maxLength={100} aria-invalid={contactField === "pb" || undefined} disabled={pending} />
            {contactField === "pb" ? contactError : null}
          </label>
          <label className="block space-y-1">
            <span className="block text-xs font-semibold text-slate-300">{L("目標タイム")}</span>
            <Input name="goal_time" value={goalTime} onChange={(e) => { setGoalTime(e.target.value); setContactField(null); }} placeholder={L("例：10秒台")} maxLength={100} aria-invalid={contactField === "goal_time" || undefined} disabled={pending} />
            {contactField === "goal_time" ? contactError : null}
          </label>
        </fieldset>

        <section aria-label={L("動画")} className="space-y-3 text-sm">
          <div className="space-y-1">
            <h2 className="font-semibold">{L("動画（{n}/{max}本・1本{seconds}秒まで）", { n: videos.length, max: maxPremiumVideos, seconds: maxReviewSeconds })}</h2>
            <p className="text-xs text-slate-300">{L("動画をテーマごとのグループに分けて送ります。グループはいくつ作っても、1つのグループに何本入れてもかまいません（練習の動画だけでも大丈夫です）。")}</p>
          </div>
          <ol className="space-y-3" data-testid="premium-groups">
            {groups.map((group, groupIndex) => {
              const titleField = `${group.key}.title`;
              const noteField = `${group.key}.focus_note`;
              return (
                <li key={group.key} data-testid="premium-group" className="space-y-3 rounded-xl border border-white/20 bg-white/5 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs font-semibold text-slate-300">{L("グループ{n}", { n: groupIndex + 1 })}</p>
                    <button type="button" className="shrink-0 text-xs text-slate-300 underline" disabled={pending}
                      onClick={() => setGroups((prev) => prev.filter((g) => g.key !== group.key))}>{L("グループを外す")}</button>
                  </div>
                  <div className="flex flex-wrap gap-2" role="group" aria-label={L("テーマ")}>
                    {premiumKinds.map((kind) => (
                      <button key={kind} type="button" aria-pressed={group.kind === kind} disabled={pending} onClick={() => changeKind(group, kind)} className={chip(group.kind === kind)}>
                        {premiumKindLabels[kind]}
                      </button>
                    ))}
                  </div>
                  <label className="block space-y-1">
                    <span className="block text-xs font-semibold text-slate-300">{L("テーマ名")}</span>
                    <Input name="title" value={group.title} onChange={(e) => updateGroup(group.key, { title: e.target.value })} maxLength={premiumGroupInputLimits.title}
                      aria-invalid={contactField === titleField || undefined} disabled={pending} />
                    {contactField === titleField ? contactError : null}
                  </label>
                  <label className="block space-y-1">
                    <span className="block text-xs font-semibold text-slate-300">{L("何を意識して取り組んでいるか・何が目的か（任意）")}</span>
                    <Textarea rows={2} name="focus_note" value={group.focus_note} onChange={(e) => updateGroup(group.key, { focus_note: e.target.value })}
                      placeholder={L("例：加速局面で前傾を保つことを意識している")} maxLength={premiumGroupInputLimits.focus_note}
                      aria-invalid={contactField === noteField || undefined} disabled={pending} />
                    {contactField === noteField ? contactError : null}
                  </label>
                  <ol className="space-y-2" data-testid="premium-videos">
                    {group.videos.map((video, index) => (
                      <li key={video.key} data-testid="premium-video" className="space-y-2 rounded-lg border border-white/15 bg-white/5 p-2">
                        <div className="flex items-center justify-between gap-2">
                          <p className="min-w-0 truncate text-xs text-slate-300">{L("{n}本目：{name}", { n: index + 1, name: video.file.name })}</p>
                          <button type="button" className="shrink-0 text-xs text-slate-300 underline" disabled={pending}
                            onClick={() => updateGroup(group.key, { videos: group.videos.filter((v) => v.key !== video.key) })}>{L("外す")}</button>
                        </div>
                        {group.kind === "race" ? raceInputs().map((input) => {
                          const field = `${group.key}.${video.key}.${input.key}`;
                          const common = {
                            name: input.key,
                            value: video[input.key],
                            placeholder: input.placeholder,
                            maxLength: premiumVideoInputLimits[input.key],
                            "aria-invalid": contactField === field || undefined,
                            disabled: pending,
                          };
                          return (
                            <label key={input.key} className="block space-y-1">
                              <span className="block text-xs font-semibold text-slate-300">{L("{label}（任意）", { label: input.label })}</span>
                              {input.multiline
                                ? <Textarea rows={2} {...common} onChange={(e) => updateVideo(group.key, video.key, { [input.key]: e.target.value })} />
                                : <Input {...common} onChange={(e) => updateVideo(group.key, video.key, { [input.key]: e.target.value })} />}
                              {contactField === field ? contactError : null}
                            </label>
                          );
                        }) : null}
                      </li>
                    ))}
                  </ol>
                  {videos.length < maxPremiumVideos ? (
                    <label className="block space-y-1">
                      <span className="font-semibold">{L("このグループに動画を追加する")}</span>
                      <input
                        className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
                        type="file"
                        accept="video/mp4,video/quicktime,video/webm"
                        value=""
                        onChange={(e) => void addVideo(group.key, e.target.files?.[0] ?? null)}
                        disabled={pending}
                      />
                    </label>
                  ) : null}
                </li>
              );
            })}
          </ol>
          {videos.length < maxPremiumVideos ? (
            <div className="space-y-2" data-testid="premium-add-group">
              <p className="font-semibold">{L("グループを追加する")}</p>
              <div className="flex flex-wrap gap-2" role="group" aria-label={L("グループを追加する")}>
                {premiumKinds.map((kind) => (
                  <button key={kind} type="button" disabled={pending} onClick={() => addGroup(kind)} className={chip(false)}>
                    {L("＋{label}", { label: premiumKindLabels[kind] })}
                  </button>
                ))}
              </div>
            </div>
          ) : null}
        </section>

        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {!estimate ? <p role="status" className="text-sm text-slate-300">{L("料金と受付状況を確認しています。表示されない場合はページを開き直してください。")}</p> : null}
        {estimate?.payments || couponsUsable ? (
          <fieldset disabled={pending} className="space-y-2">
            <legend className="text-sm font-semibold">{L("お支払い方法")}</legend>
            <div className="flex flex-wrap gap-2">
              <button type="button" aria-pressed={method === "card"} onClick={() => setMethod("card")} className={chip(method === "card")}>
                {L("{v0}／回", { v0: formatDisplayPrice(priceYen) })}
              </button>
              {couponsUsable ? (
                <button type="button" aria-pressed={method === "coupon"} onClick={() => setMethod("coupon")} className={chip(method === "coupon")}>
                  {L("¥500クーポン（残り {n} 枚）", { n: couponsLeft })}
                </button>
              ) : null}
            </div>
            {method === "card" ? <p className="text-xs text-slate-300">{reviewPayMethodLabels.card}</p> : (
              <div className="space-y-1 text-sm" aria-label={L("クーポンの内訳")}>
                <p>{L("価格 {price} − クーポン {coupon} ＝ お支払い {pay}", { price: formatDisplayPrice(priceYen), coupon: formatDisplayPrice(couponYen), pay: formatDisplayPrice(cashPrice) })}</p>
                {ticketSummary?.coupons?.next_expires_at ? <p className="text-xs text-slate-300">{L("使うクーポンの有効期限：{v0}", { v0: formatDateTime(ticketSummary.coupons.next_expires_at) })}</p> : null}
              </div>
            )}
            <p className="text-xs text-slate-300">{L("プレミアムはチケットでは申し込めません。")}</p>
          </fieldset>
        ) : null}
        {estimate ? <p className="text-sm text-slate-100">{L("申し込みが確定してから24時間以内に、グループごとの回答とまとめが届きます。")}</p> : null}
        <p className="text-xs text-slate-300">{L("期限までに回答が届かなかった場合は、お支払いを取り消すか全額を返金します。チケット・クーポンは返還します。")}</p>
        {needsDeclaration ? <MinorPurchaseDeclaration checked={declared} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} /> : null}
        <Button type="submit" className="w-full bg-[#0d3d8f] hover:bg-[#0a49c2] active:bg-[#0c2654]" disabled={pending || !estimate || videos.length === 0 || (needsDeclaration && !declared)}>
          {pending ? L("送っています") : L("申込み内容を確認する")}
        </Button>
      </form>
    </div>
  );
}
