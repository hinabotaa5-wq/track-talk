"use client";

import { PageLoading } from "@/components/page-loading";

import { useEffect, useState, type ReactNode } from "react";
import { apiFetch } from "@/lib/api";
import { coachingEnabled } from "@/lib/features";
import { inquiryCategoryLabels } from "@/lib/validators";

type WeekRow = {
  week_start: string;
  signups: number;
  active_users: number;
  posts: number;
  sense_notes: number;
  race_records: number;
  deletions: number;
};

type SenseNoteStats = {
  total: number;
  authors: number;
  non_admin_authors: number;
  profiles: number;
  non_admin_profiles: number;
  first_day: string | null;
  last_day: string | null;
  buckets: { one: number; two_four: number; five_nine: number; ten_plus: number };
  active_days: { days_2_plus: number; days_3_plus: number; days_7_plus: number };
  repeat_7: { total: number; repeat: number };
  repeat_28: { total: number; repeat: number };
  cohorts: { week_start: string; authors: number; next_week: number; since_next_week: number }[];
  weekly: { week_start: string; notes: number; authors: number }[];
  monthly: { month: string; notes: number; authors: number }[];
};

type AdminStats = {
  sense_notes?: SenseNoteStats | null;
  daily_started_at?: string;
  daily?: {
    day: string;
    signups: number;
    posts: number;
    comments: number;
    likes: number;
    sense_notes: number;
    race_records: number;
    product_reviews: number;
    deletions: number;
  }[];
  recent_days: number;
  weeks: WeekRow[];
  retention: { week_start: string; signups: number; retained: number }[];
  features: { feature: string; recent: number; recent_users: number; total: number }[] | null;
  inquiries: { source: string; category: string; recent: number; total: number }[] | null;
  reports: { target_type: string; reason: string; recent: number; open: number; total: number }[] | null;
};

type CoachingStats = {
  days: number;
  events: {
    event: string;
    event_label: string;
    coaches: number;
    on_duty: number;
    requests: number;
    returned: number;
    expired: number;
    median_hours: number;
    rated: number;
    avg_stars: number;
    helpful_answers: number;
    helpful_yes: number;
    applied: number;
    started: number;
    lost: number;
    started_yen: number;
  }[];
};

const featureLabels: Record<string, string> = {
  comment: "コメント",
  save: "保存",
  sense_note: "感覚メモ",
  product_review: "シューズのレビュー",
  practice_application: "練習への参加申請",
  injury_group: "ケガのグループに参加",
  menu_contest_entry: "エグ練グランプリの応募",
  menu_contest_vote: "エグ練グランプリの投票",
};

const sourceLabels: Record<string, string> = {
  contact: "お問い合わせ",
  feedback: "なんでも書いてね",
  product_request: "商品リクエスト",
};

const targetLabels: Record<string, string> = {
  post: "投稿",
  comment: "コメント",
  injury_note: "ケガ部の一言",
  review_response: "添削の返答",
  coaching_message: "パーソナル指導のやりとり",
  practice_rating: "練習の評価のひとこと",
};

const reasonLabels: Record<string, string> = {
  spam: "スパム",
  harassment: "嫌がらせ",
  misinfo: "誤情報",
  other: "その他",
};

function percent(part: number, whole: number) {
  return whole > 0 ? `${Math.round((part / whole) * 100)}%` : "-";
}

function weekLabel(day: string) {
  const [, m, d] = day.split("-");
  return `${Number(m)}/${Number(d)}〜`;
}

function Section({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3">
      <h2 className="text-base font-semibold">{title}</h2>
      {note ? <p className="text-xs text-slate-300">{note}</p> : null}
      <div className="overflow-x-auto">{children}</div>
    </section>
  );
}

function Table({ head, rows }: { head: string[]; rows: (string | number)[][] }) {
  if (rows.length === 0) return <p className="text-sm text-slate-300">まだありません</p>;
  return (
    <table className="w-full min-w-max text-sm">
      <thead>
        <tr className="text-left text-xs text-slate-300">
          {head.map((h, i) => (
            <th key={h} className={`px-2 py-1 font-medium ${i > 0 ? "text-right" : ""}`}>
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i} className="border-t border-white/10">
            {row.map((cell, j) => (
              <td key={j} className={`px-2 py-1 ${j > 0 ? "text-right tabular-nums" : ""}`}>
                {cell}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// 管理者画面の「数字」タブ。件数と人数だけで、だれが何を書いたかは出さない。
export function AdminStatsView({ token }: { token: string | null }) {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [coaching, setCoaching] = useState<CoachingStats | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    Promise.all([
      apiFetch<AdminStats>("/v1/admin/stats", { token }),
      apiFetch<CoachingStats>("/v1/admin/stats/coaching", { token }),
    ])
      .then(([s, c]) => {
        if (!active) return;
        setStats(s);
        setCoaching(c);
        setError("");
      })
      .catch(() => {
        if (active) setError("読み込めませんでした");
      });
    return () => {
      active = false;
    };
  }, [token]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (!stats || !coaching) return <PageLoading label="読み込み中..." />;

  const days = stats.recent_days;
  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h2 className="text-lg font-bold">アプリ内の利用状況</h2>
        <p className="text-sm text-slate-300">アカウント単位の利用人数・継続率と、Umamiに送っていない機能の利用・運営状況を表示します。閲覧数・訪問者数・投稿・いいね・競技会の記録の件数はUmamiで確認できます。</p>
      </div>
      {stats.daily_started_at ? (
        <Section
          title="日ごとに残す件数"
          note={`日本時間で直近90日を表示。${new Date(stats.daily_started_at).toLocaleString("ja-JP", { timeZone: "Asia/Tokyo" })}から保存しています。初日は開始後の分だけ、今日は途中の件数です。新規保存に成功した回数を数え、編集は含めません。削除・退会後も件数は減りません。`}
        >
          <Table
            head={["日付", "コメント", "感覚メモ", "レビュー", "退会"]}
            rows={(stats.daily ?? []).map((d) => [d.day, d.comments, d.sense_notes, d.product_reviews, d.deletions])}
          />
        </Section>
      ) : null}
      <Section
        title="週ごとの動き"
        note="週は日本時間の月曜始まり。「書いた人数」は投稿・コメント・いいね・感覚メモ・記録・レビュー・練習の申請・ケガのグループへの参加のどれかをしたアカウント数です。同じアカウントは1人として数えます。読むだけの人は含みません。退会した人のぶんは過去の週からも消えます。"
      >
        <Table
          head={["週", "書いた人数", "感覚メモ", "退会"]}
          rows={stats.weeks.map((w) => [weekLabel(w.week_start), w.active_users, w.sense_notes, w.deletions])}
        />
      </Section>

      <Section title="続けて使っている人" note="登録した週ごとに、登録から29〜35日目(5週目)にも書き込みをしたアカウントの割合。同じアカウントの利用を端末をまたいで集計します。5週目が終わった週だけ出します。">
        <Table
          head={["登録した週", "登録", "5週目も書いた", "割合"]}
          rows={stats.retention.map((r) => [weekLabel(r.week_start), r.signups, r.retained, percent(r.retained, r.signups)])}
        />
      </Section>

      {stats.sense_notes ? (
        <>
          <Section
            title="感覚メモの全体"
            note="件数・人数・日付だけの集計で、本文やだれが書いたかは出ません。削除・退会のぶんは減ったあとの現在値です。"
          >
            <Table
              head={["項目", "件数・人数・日付"]}
              rows={[
                ["総件数", stats.sense_notes.total],
                ["利用者数", stats.sense_notes.authors],
                ["非管理者の利用者数", stats.sense_notes.non_admin_authors],
                ["全登録者数", stats.sense_notes.profiles],
                ["非管理者の登録者数", stats.sense_notes.non_admin_profiles],
                ["最初の記録日", stats.sense_notes.first_day ?? "-"],
                ["最後の記録日", stats.sense_notes.last_day ?? "-"],
              ]}
            />
          </Section>

          <Section title="感覚メモの1人あたり件数" note="人数だけの分布です。">
            <Table
              head={["1人あたり", "人数"]}
              rows={[
                ["1件", stats.sense_notes.buckets.one],
                ["2〜4件", stats.sense_notes.buckets.two_four],
                ["5〜9件", stats.sense_notes.buckets.five_nine],
                ["10件以上", stats.sense_notes.buckets.ten_plus],
              ]}
            />
          </Section>

          <Section title="感覚メモの書いた日数" note="同じ日に複数件書いても1日と数えます。人数だけの分布です。">
            <Table
              head={["書いた日数", "人数"]}
              rows={[
                ["2日以上", stats.sense_notes.active_days.days_2_plus],
                ["3日以上", stats.sense_notes.active_days.days_3_plus],
                ["7日以上", stats.sense_notes.active_days.days_7_plus],
              ]}
            />
          </Section>

          <Section
            title="感覚メモの初回からのリピート"
            note="初回日から7日・28日以内に2回目を書いた人の割合。分母は期間が経過した人だけで、直近に始めた人では薄まりません。対象が0人のときは「まだ測れない」と出します。"
          >
            <Table
              head={["期間", "リピート", "対象", "割合"]}
              rows={[stats.sense_notes.repeat_7, stats.sense_notes.repeat_28].map((r, i) => [
                i === 0 ? "7日以内" : "28日以内",
                r.repeat,
                r.total,
                r.total > 0 ? percent(r.repeat, r.total) : "まだ測れない",
              ])}
            />
          </Section>

          <Section
            title="感覚メモの週次コホート"
            note="初回日の属する週（月曜始まり）ごとに、翌週（初回週+7日〜+14日未満）と翌週以降（初回週+7日以降）に書いた人数です。"
          >
            <Table
              head={["初回の週", "人数", "翌週も書いた", "翌週以降に書いた"]}
              rows={stats.sense_notes.cohorts.map((c) => [
                weekLabel(c.week_start),
                c.authors,
                `${c.next_week}（${percent(c.next_week, c.authors)}）`,
                c.since_next_week,
              ])}
            />
          </Section>

          <Section title="感覚メモの週ごとの推移" note="書いた日（日本時間の日付）の週で分けた件数と利用者数です。">
            <Table
              head={["週", "件数", "利用者数"]}
              rows={stats.sense_notes.weekly.map((w) => [weekLabel(w.week_start), w.notes, w.authors])}
            />
          </Section>

          <Section title="感覚メモの月ごとの推移" note="書いた日（日本時間の日付）の月で分けた件数と利用者数です。">
            <Table
              head={["月", "件数", "利用者数"]}
              rows={stats.sense_notes.monthly.map((m) => [m.month.slice(0, 7), m.notes, m.authors])}
            />
          </Section>
        </>
      ) : null}

      <Section title="機能ごとの利用" note={`Umamiに送っていない機能の、直近${days}日の件数・利用アカウント数と、現在残っている全期間の件数。削除や取り消しで件数は減ります。`}>
        <Table
          head={["機能", `${days}日の件数`, `${days}日の人数`, "全期間の件数"]}
          rows={(stats.features ?? []).filter((f) => Object.hasOwn(featureLabels, f.feature)).map((f) => [featureLabels[f.feature], f.recent, f.recent_users, f.total])}
        />
      </Section>

      <Section title="問い合わせの内訳" note={`直近${days}日とこれまでの件数。`}>
        <Table
          head={["窓口", "種類", `${days}日`, "これまで"]}
          rows={(stats.inquiries ?? []).map((q) => [
            sourceLabels[q.source] ?? q.source,
            inquiryCategoryLabels[q.category as keyof typeof inquiryCategoryLabels] ?? q.category,
            q.recent,
            q.total,
          ])}
        />
      </Section>

      <Section title="通報の内訳" note={`直近${days}日、未対応、これまでの件数。`}>
        <Table
          head={["対象", "理由", `${days}日`, "未対応", "これまで"]}
          rows={(stats.reports ?? []).map((r) => [targetLabels[r.target_type] ?? r.target_type, reasonLabels[r.reason] ?? r.reason, r.recent, r.open, r.total])}
        />
      </Section>

      <Section
        title="コーチングの需要と供給(種目ごと)"
        note={`コーチは承認済みの人数(1人が複数の種目を担当するので、合計は人数より多くなります)。依頼と申し込みは直近${coaching.days}日。${
          coachingEnabled ? "" : "提供前なので、デモの招待コードで試した分だけが入ります。"
        }`}
      >
        <Table
          head={["種目", "コーチ", "受付中", "添削の依頼", "返した", "期限切れ", "返すまで(中央値)", "役に立った", "指導の申し込み", "始まった", "成立しなかった", "始まった金額"]}
          rows={coaching.events.map((e) => [
            e.event_label,
            e.coaches,
            e.on_duty,
            e.requests,
            e.returned,
            e.expired,
            e.returned > 0 ? `${e.median_hours.toFixed(1)}時間` : "-",
            percent(e.helpful_yes, e.helpful_answers),
            e.applied,
            e.started,
            e.lost,
            `${e.started_yen.toLocaleString("ja-JP")}円`,
          ])}
        />
      </Section>
    </div>
  );
}
