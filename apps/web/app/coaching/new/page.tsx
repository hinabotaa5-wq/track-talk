"use client";
import { ReferralInput, validReferralInput } from "@/components/coaching-referral";
import { L } from "@/lib/i18n";

import { useSolicitationGuard } from "@/components/contact-notice";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { DemoCodeForm } from "@/components/coaching-demo";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { PurchaseConfirm } from "@/components/purchase-confirm";
import { TicketBalance } from "@/components/ticket-balance";
import { ArrowRight } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, apiErrorText, ApiRequestError } from "@/lib/api";
import { loginPathFrom } from "@/lib/safe-next";
import { useMinorDeclaration } from "@/lib/minor-purchase";
import { coachEventLabels } from "@/lib/coach-recruit";
import { eagerVideoQuery, uploadVideoToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import {
  maxReviewSeconds,
  reviewEvents,
  reviewPayMethodLabels,
  tooLong,
  videoDuration,
} from "@/lib/review";
import { initialReviewPayment, parseReviewPayParam, ticketExpiryFor, ticketKindCounts, type TicketKind, type TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import { reviewPriceYen, ticketPackPriceYen, ticketPackQuantity, ticketValidityMonths } from "@/lib/review-pricing";
import { reviewConfirmRows } from "@/lib/purchase-confirm";
import type { CoachEvent, CoachingMe, ReviewCoachOption, ReviewEstimate, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { looksLikeContact } from "@/lib/solicitation";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

// フォーム添削の依頼を送る。支払いのある環境では誰でも申し込める（招待コードのゲートは外した。本体 #569）。
// 依頼するコーチは選手が選ぶ（運営からの割り当てはしない）。料金はどのコーチでも同じ。
// 送る前に最終確認（内容・回答期限・お支払い／総額・キャンセルと戻り／返金の条件）を出し、
// 確定を押してから動画を上げて、カードはそのまま支払い画面へ進む。支払いが済んだら選んだコーチに依頼が届く。
// Stripe の鍵がない検証用の環境では、支払いなしのデモ版（招待コードを入れた人だけ）。

const maxBytes = 100 * 1024 * 1024;

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export default function NewReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const tier = "coach";
  const [event, setEvent] = useState<CoachEvent>(reviewEvents[0]);
  const [question, setQuestion] = useState("");
  const [referralCode, setReferralCode] = useState("");
  const [referralWarning, setReferralWarning] = useState(false);
  const [contactBlocked, setContactBlocked] = useState(false);
  const guard = useSolicitationGuard();
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  // 18歳未満の購入のたびの宣言。毎回チェックなしから始める。
  const declaration = useMinorDeclaration();
  const declared = declaration.checked;
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [estimate, setEstimate] = useState<ReviewEstimate | null>(null);
  const [ticketSummary, setTicketSummary] = useState<TicketSummary | null>(null);
  const [method, setMethod] = useState<ReviewPayMethod | "tickets">("card");
  // チケットの内訳（通常／無料。本体 #687）。種別が分からない古い API では null のまま（従来どおり自動）。
  const [ticketKind, setTicketKind] = useState<TicketKind | null>(null);
  const { pending, acquire, release } = useActionLock();
  const [coaches, setCoaches] = useState<ReviewCoachOption[]>([]);
  const [coachID, setCoachID] = useState("");
  const [coachesLoading, setCoachesLoading] = useState(true);
  const [coachesError, setCoachesError] = useState("");
  // コーチ詳細からの指定（?coach=）。受付中なら初期選択し、受付中でなければ明示を出す（指定は捨てない）。
  const [coachNotice, setCoachNotice] = useState("");
  const appliedCoachRef = useRef("");
  const coach = coaches.find((c) => c.id === coachID);
  const totalPrice = estimate?.price_yen ?? reviewPriceYen;
  const cashPrice = method === "tickets" ? 0 : totalPrice;
  // チケットが1枚（添削1回分）以上残っている人にだけ、チケットで払う選択肢を出す。
  const ticketsUsable = (ticketSummary?.balance.available ?? 0) >= 1;
  const kindCounts = ticketKindCounts(ticketSummary);
  const chosenKind = method === "tickets" ? ticketKind : null;
  const chosenExpiry = ticketExpiryFor(ticketSummary, chosenKind);
  const cashPayment = !!estimate?.payments && cashPrice > 0;
  // 18歳未満が自分で払うときだけ、宣言のチェックを出して必須にする。
  const needsDeclaration = cashPayment;
  // 申し込めるか。古い API（review_open を返さない）では、これまでどおり招待コードの有無で見る。
  const reviewOpen = !!(me?.review_open ?? me?.demo);
  // 最終確認を出しているか。お金かチケットが動く申込みだけ確認を挟む（デモ版はそのまま送る）。
  const [confirming, setConfirming] = useState(false);
  const needsConfirm = cashPayment || method === "tickets";

  useEffect(() => {
    if (!token || !reviewOpen) return;
    let cancelled = false;
    apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token })
      .then((data) => { if (!cancelled) setCoaches(data.coaches); })
      .catch(() => { if (!cancelled) setCoachesError(L("コーチの一覧を読み込めませんでした。ページを開き直してください。")); })
      .finally(() => { if (!cancelled) setCoachesLoading(false); });
    return () => { cancelled = true; };
  }, [event, token, reviewOpen]);

  useEffect(() => {
    if (!loading && !session) {
      // ログインを挟んでも coach 指定が落ちないよう、クエリを含めた戻り先にする。
      router.replace(loginPathFrom(window.location));
    }
  }, [loading, session, router]);

  // コーチ詳細からの ?coach= の引き継ぎ。受付中（いずれかの担当種目で nominees に含まれる）なら
  // そのコーチを初期選択し、初期の種目はコーチの担当種目の並び順を優先する。
  // 受付中でない場合は明示を出し、他のコーチを選べる状態にする。coach 指定なしの従来動作は変えない。
  useEffect(() => {
    const requested = new URLSearchParams(window.location.search).get("coach")?.trim() ?? "";
    if (!token || !reviewOpen || !requested || appliedCoachRef.current === requested) return;
    let cancelled = false;
    void (async () => {
      const lists = await Promise.all(reviewEvents.map(async (reviewEvent) => {
        try {
          const data = await apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(reviewEvent)}`, { token });
          return { event: reviewEvent, coaches: data.coaches as ReviewCoachOption[] | null };
        } catch {
          // 取得に失敗した種目は判定に使わない。全種目で失敗したら何もしない。
          return { event: reviewEvent, coaches: null as ReviewCoachOption[] | null };
        }
      }));
      if (cancelled) return;
      if (lists.every(({ coaches: list }) => list === null)) return;
      appliedCoachRef.current = requested;
      const accepting = lists.filter(({ coaches: list }) => (list ?? []).some((c) => c.id === requested));
      if (accepting.length === 0) {
        setCoachNotice(L("このコーチは現在、動画添削を受け付けていません"));
        return;
      }
      const record = accepting.flatMap(({ coaches: list }) => list ?? []).find((c) => c.id === requested);
      const acceptingEvents = new Set(accepting.map(({ event: reviewEvent }) => reviewEvent));
      const preferred = record?.events.find((e) => acceptingEvents.has(e)) ?? accepting[0].event;
      setEvent(preferred);
      setCoaches(lists.find(({ event: reviewEvent }) => reviewEvent === preferred)?.coaches ?? []);
      setCoachesLoading(false);
      setCoachesError("");
      setCoachID(requested);
      setCoachNotice("");
    })();
    return () => { cancelled = true; };
  }, [token, reviewOpen]);

  // 種目切替時の整合：指定コーチが今の種目で受付中にいない（切替後など）ときも、
  // 送信を止めて枠なしの注意で分かるようにする（state を増やさず描画時に判定する）。
  const coachInactiveForEvent = !!coachID && !coachesLoading && !coachesError && !coaches.some((c) => c.id === coachID);
  const visibleCoachNotice = coachNotice || (coachInactiveForEvent ? L("このコーチは現在、動画添削を受け付けていません") : "");

  async function loadMe() {
    try {
      setMe(await apiFetch<CoachingMe>("/v1/me/coaching", { token }));
    } catch {
      setLoadError(L("読み込みに失敗しました"));
    }
  }

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((d) => {
        if (!cancelled) setMe(d);
      })
      .catch(() => {
        if (!cancelled) setLoadError(L("読み込みに失敗しました"));
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token }).then((data) => {
      if (cancelled) return;
      setTicketSummary(data);
      // コーチ詳細の入口（?pay=card|normal|reward）を初期値にする。使えない種別なら使える側に直す（本体 #687）。
      const initial = initialReviewPayment(data, parseReviewPayParam(new URLSearchParams(window.location.search).get("pay")));
      setMethod(initial.method);
      setTicketKind(initial.kind);
    }).catch(() => { if (!cancelled) setTicketSummary(null); });
    return () => { cancelled = true; };
  }, [token]);

  // 枠と種目に対応する料金・支払い情報を取得する。
  useEffect(() => {
    if (!token || !reviewOpen) return;
    let cancelled = false;
    apiFetch<ReviewEstimate>(`/v1/coaching/estimate?tier=${tier}&event=${encodeURIComponent(event)}`, { token })
      .then((d) => {
        if (!cancelled) setEstimate(d);
      })
      .catch(() => {
        if (!cancelled) setEstimate(null);
      });
    return () => {
      cancelled = true;
    };
  }, [token, tier, event, reviewOpen]);

  async function onPickFile(picked: File | null) {
    setError("");
    setFile(picked);
    if (!picked) return;
    if (tooLong(await videoDuration(picked))) {
      setError(L("動画は{maxReviewSeconds}秒以内にしてください", { maxReviewSeconds }));
      setFile(null);
    }
  }

  // 入力を確かめて、最終確認を出す（確認のいらないデモ版はそのまま送る）。
  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (looksLikeContact(question)) {
      setError("");
      setContactBlocked(true);
      const input = e.currentTarget.elements.namedItem("question");
      if (input instanceof HTMLTextAreaElement) input.focus();
      return;
    }
    if (!validate()) return;
    if (needsConfirm) { setConfirming(true); return; }
    void send();
  }

  function validate(): boolean {
    if (!estimate) { setError(L("料金と受付状況を確認してから送信してください。")); return false; }
    if (!coach) { setError(L("コーチを選んでください。")); return false; }
    if (question.trim() && !guard.allow(question)) return false;
    if (!question.trim()) {
      setError(L("相談内容を記入してください"));
      return false;
    }
    if (!file) {
      setError(L("動画を選んでください"));
      return false;
    }
    if (file.size > maxBytes) {
      setError(L("100MB以内の動画にしてください"));
      return false;
    }
    if (method === "tickets" && (ticketSummary?.balance.available ?? 0) < 1) {
      setError(L("チケットが不足しています。先に購入してから動画を選んでください。"));
      return false;
    }
    if (chosenKind && kindCounts && kindCounts[chosenKind] < 1) {
      setError(L("選んだチケットの残りがありません。別のお支払い方法を選んでください。"));
      return false;
    }
    if (needsDeclaration && !declared) {
      setError(L("購入前の注意事項を確認して、チェックを入れてください。"));
      return false;
    }
    setError("");
    return true;
  }

  async function send() {
    if (!file || !coach || !validate() || !acquire()) return;
    setError("");
    try {
      if (cashPayment && !await validReferralInput(referralCode, token)) {
        setReferralCode("");
        setReferralWarning(true);
        return;
      }
      const signed = await apiFetch<CloudinarySignature>(`/v1/uploads/coaching-video${eagerVideoQuery}`, { method: "POST", token });
      setUploadPhase("upload");
      setUpload({ loaded: 0, total: file.size, percent: 0 });
      const payload = await uploadVideoToCloudinary(file, signed, token, setUpload);
      if (payload.public_id !== signed.public_id) throw new Error(L("アップロードに失敗しました"));
      setUploadPhase("saving");
      const created = await apiFetch<ReviewRequest>("/v1/coaching/requests", {
        method: "POST",
        token,
        body: JSON.stringify({ tier, event, question: question.trim(), video_public_id: payload.public_id,
          method, use_tickets: method === "tickets", ticket_kind: chosenKind ?? undefined, referral_code: cashPayment ? referralCode : undefined, requested_coach_id: coach.id, ...declaration.body(needsDeclaration) }),
      });
      if (created.checkout_url) {
        window.location.assign(created.checkout_url);
        return;
      }
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : L("送信に失敗しました")));
      if (err instanceof ApiRequestError && ["ticket_kind_unavailable", "insufficient_tickets"].includes(err.code)) {
        // 残りを読み直す。選んだ種別は他に切り替えない（0枚の側は選べない表示になる）。
        try { setTicketSummary(await apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token })); } catch { /* 元のエラーを表示する。 */ }
      }
      if (err instanceof ApiRequestError && ["nomination_changed", "validation_error"].includes(err.code)) {
        try { const data = await apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token }); setCoaches(data.coaches); } catch { /* 元のエラーを表示して再読込を促す。 */ }
      }
      setUpload(null);
      setConfirming(false);
      release();
    }
  }

  if (loading || !session || (!me && !loadError)) {
    return <PageLoading screen label={L("読み込み中...")} />;
  }
  if (!me) {
    return <p className="text-sm text-red-400">{loadError}</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        {!estimate || estimate.payments ? null : (
          <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">{L("デモ版")}</p>
        )}
        <h1 className="text-2xl font-bold">{L("動画添削を頼む")}</h1>
      </div>

      {!reviewOpen ? (
        <DemoCodeForm
          intro={L("フォーム添削は準備中です。いまはデモ版として、運営から招待コードを受け取った人だけが使えます。")}
          token={token}
          onJoined={() => void loadMe()}
        />
      ) : confirming ? (
        <div className="space-y-3">
          <PurchaseConfirm
            rows={reviewConfirmRows({ method, priceYen: totalPrice, coachName: coach?.nickname ?? undefined, nextExpiry: chosenExpiry ? { expires_at: chosenExpiry } : null, ticketKind: chosenKind })}
            confirmLabel={chosenKind === "reward" ? L("確定して無料チケット1枚で送る") : method === "tickets" ? L("確定してチケット1枚で送る") : L("確定してお支払いへ（{v0}）", { v0: formatYen(cashPrice) })}
            onConfirm={() => void send()}
            onBack={() => { setConfirming(false); setError(""); }}
            pending={pending}
          >
            {cashPayment ? <ReferralInput code={referralCode} warning={referralWarning} disabled={pending}
              onChange={(code) => { setReferralCode(code); setReferralWarning(false); }} /> : null}
            <Link href="/legal/commerce" className="inline-block text-xs text-slate-200 underline">
              {L("特定商取引法に基づく表記")}
            </Link>
            {method === "tickets" ? (
              <p className="text-xs text-slate-300">{L("当社・コーチ都合で受けられなかった場合の返金は、利用規約第6条のとおりです。")}</p>
            ) : null}
          </PurchaseConfirm>
          {upload ? (
            <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} />
          ) : null}
          {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-5">
          <fieldset disabled={pending} className="space-y-2">
            <legend className="text-sm font-semibold">{L("種目")}</legend>
            <div className="flex flex-wrap gap-2">
              {reviewEvents.map((value) => (
                <button key={value} type="button" aria-pressed={event === value} onClick={() => { if (value !== event) { setEvent(value); setCoachesLoading(true); setCoachesError(""); setCoaches([]); } }} className={chip(event === value)}>
                  {coachEventLabels[value]}
                </button>
              ))}
            </div>
          </fieldset>

          {visibleCoachNotice ? <p role="note" className="text-sm text-amber-200">{visibleCoachNotice}</p> : null}
          {coachesError ? <p role="alert" className="text-sm text-red-400">{coachesError}</p> : null}
          {!coachID ? <p className="text-sm"><Link href="/coaches" className="text-blue-300 underline">{L("コーチ一覧")}</Link></p> : null}
          <div className="space-y-2 text-sm">
            <label htmlFor="review-question" className="block font-semibold">{L("相談内容")}</label>
            {!contactBlocked && guard.warning}
            <Textarea
              id="review-question"
              name="question"
              value={question}
              onChange={(e) => { setQuestion(e.target.value); setContactBlocked(false); }}
              aria-invalid={contactBlocked || undefined}
              aria-describedby={contactBlocked ? "review-question-contact-error" : undefined}
              rows={4}
              maxLength={1000}
              required
              disabled={pending}
            />
            {contactBlocked ? (
              <span id="review-question-contact-error" role="alert" className="block text-sm text-red-400">
                {L("外部の連絡先（URL・メール・SNS IDなど）は送れません。")}
              </span>
            ) : null}
            <span className="block text-xs text-slate-300">{L("{length}/1000文字", { length: question.length })}</span>
          </div>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">{L("動画({maxReviewSeconds}秒まで)", { maxReviewSeconds })}</span>
            <input
              className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="video/mp4,video/quicktime,video/webm"
              onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
              disabled={pending}
            />
          </label>

          {upload ? (
            <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} />
          ) : null}
          {error ? <p className="text-sm text-red-400">{error}</p> : null}
          {!estimate ? <p role="status" className="text-sm text-slate-300">{L("料金と受付状況を確認しています。表示されない場合はページを開き直してください。")}</p> : null}
          {estimate ? (
            <p className="text-sm text-slate-100">
              {L("申し込みが確定してから24時間以内に回答が来ます。")}
            </p>
          ) : null}
          {coachID ? <p className="text-xs text-slate-300">{L("受付中のコーチには決済の確認後に依頼が届きます。期限までに添削が届かなかった場合は、お支払いを取り消すか全額を返金します。チケットは返還します。")}</p> : null}
          {estimate?.payments || ticketsUsable ? (
            <fieldset disabled={pending} className="space-y-2">
              <legend className="text-sm font-semibold">{L("お支払い方法")}</legend>
              <div className="flex flex-wrap gap-2">
                <button type="button" aria-pressed={method === "card"} onClick={() => setMethod("card")} className={chip(method === "card")}>
                  {L("単発 {v0}／回", { v0: formatYen(totalPrice) })}
                </button>
                {ticketsUsable ? (
                  <button type="button" aria-pressed={method === "tickets"} onClick={() => setMethod("tickets")} className={chip(method === "tickets")}>
                    {L("チケット（6枚）{v0}", { v0: formatYen(ticketPackPriceYen) })}
                  </button>
                ) : ticketSummary?.can_purchase ? (
                  <Link href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew" className={chip(false)}>
                    {L("チケット（6枚）{v0}", { v0: formatYen(ticketPackPriceYen) })}
                  </Link>
                ) : null}
              </div>
              {method === "card" ? (
                <p className="text-xs text-slate-300">{reviewPayMethodLabels.card}</p>
              ) : null}
              {method === "tickets" && ticketsUsable && ticketSummary ? (
                <div className="space-y-2 text-sm">
                  {kindCounts ? (
                    <div className="flex flex-wrap gap-2" role="group" aria-label={L("使うチケット")}>
                      {(["normal", "reward"] as const).map((kind) => {
                        const left = kindCounts[kind];
                        const rewardExpiry = ticketSummary.balance_by_kind?.reward.next_expires_at;
                        const label = kind === "normal"
                          ? L("通常チケット（残り {n} 枚）", { n: left })
                          : left >= 1 && rewardExpiry
                            ? L("無料チケット（残り {n} 枚・いちばん早い有効期限 {expiry}）", { n: left, expiry: formatDateTime(rewardExpiry) })
                            : L("無料チケット（残り {n} 枚）", { n: left });
                        return (
                          <button key={kind} type="button" aria-pressed={ticketKind === kind} disabled={left < 1} onClick={() => setTicketKind(kind)}
                            className={`${chip(ticketKind === kind)} disabled:cursor-not-allowed disabled:opacity-50`}>
                            {label}
                          </button>
                        );
                      })}
                    </div>
                  ) : null}
                  <TicketBalance summary={ticketSummary} />
                  <p className="text-xs text-slate-300">{L("動画添削1回はチケット1枚です。")}</p>
                  <Link href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew" className="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-white/15 active:scale-[0.98]">
                    {ticketSummary.can_purchase ? L("チケットを購入・確認する") : L("コーチングチケット")}<ArrowRight className="size-4 text-blue-300" aria-hidden />
                  </Link>
                </div>
              ) : null}
              {!ticketsUsable && ticketSummary?.can_purchase ? (
                <p className="text-xs text-slate-300">{L("1回あたり約{v0}（1回ずつ払うと{v1}）。先払い・{q}枚まとめての購入で、有効期限は決済から{m}か月です。", { v0: formatYen(Math.round(ticketPackPriceYen / ticketPackQuantity)), v1: formatYen(reviewPriceYen), q: ticketPackQuantity, m: ticketValidityMonths })}</p>
              ) : null}
            </fieldset>
          ) : (
            <p className="text-xs text-slate-300">{L("デモ版のため料金はかかりません。目安までに返ってこなかった依頼は期限切れになります。")}</p>
          )}
          {needsDeclaration ? <MinorPurchaseDeclaration checked={declared} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} /> : null}
          <Button type="submit" className="w-full bg-[#0d3d8f] hover:bg-[#0a49c2] active:bg-[#0c2654]" disabled={pending || !file || !estimate || !coach || (needsDeclaration && !declared)}>
            {pending ? L("送っています") : needsConfirm ? L("申込み内容を確認する") : L("送る")}
          </Button>
        </form>
      )}
    </div>
  );
}
