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

type AdminStats = {
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
  menu_contest_entry: "エグ練の応募",
  menu_contest_vote: "エグ練の投票",
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
          head={["種目", "コーチ", "受付中", "添削の依頼", "返した", "期限切れ", "返すまで(中央値)", "★平均", "役に立った", "指導の申し込み", "始まった", "成立しなかった", "始まった金額"]}
          rows={coaching.events.map((e) => [
            e.event_label,
            e.coaches,
            e.on_duty,
            e.requests,
            e.returned,
            e.expired,
            e.returned > 0 ? `${e.median_hours.toFixed(1)}時間` : "-",
            e.rated > 0 ? `${e.avg_stars.toFixed(1)}(${e.rated}件)` : "-",
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
