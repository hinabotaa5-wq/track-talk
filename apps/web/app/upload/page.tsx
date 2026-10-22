"use client";

import Link from "next/link";
import { FileText, Handshake, Link2, Users, Video } from "lucide-react";
import { useAuth } from "@/components/auth-provider";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 5つが同じ色だと見分けにくいので、種類ごとにアイコンと色を分ける。
const doors = [
  { href: "/posts/new", title: "文章", body: "詳細・写真のいずれかでスレッドを立てます。", icon: FileText, border: "border-l-sky-400", tile: "bg-sky-500/20 text-sky-300" },
  { href: "/posts/url", title: "動画URL", body: "YouTubeやInstagramの動画を引用して、そのスレッドに集まります。", icon: Link2, border: "border-l-violet-400", tile: "bg-violet-500/20 text-violet-300" },
  { href: "/posts/video", title: "自分の動画", body: "自分の走った動画を投稿します。", icon: Video, border: "border-l-rose-400", tile: "bg-rose-500/20 text-rose-300" },
  { href: "/posts/recruit", title: "クラブチームの募集", body: "チーム名や活動場所を書いて、メンバーを募ります。", icon: Users, border: "border-l-emerald-400", tile: "bg-emerald-500/20 text-emerald-300" },
  { href: "/posts/practice", title: "練習相手募集", body: "練習場所と日時を書いて、一緒に練習する相手を見つけます。", icon: Handshake, border: "border-l-amber-400", tile: "bg-amber-500/20 text-amber-300" },
] as const;

export default function UploadPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中…</p>;

  // ホームの「＋」から開く詳細画面。下のメニューは出さないので、その分まで5つの入口で埋める。
  // dvh だと、iPhone の Safari で画面を開いた直後に下のツールバーが出てきた分だけ高さが縮み、
  // 5つの入口が上にギュッと詰まる。ツールバーが出ている時の高さ(svh)で固定して動かさない。
  // 高さ = 画面 - ヘッダー(3.8125rem) - 上の余白(1.5rem) - 下の余白(0.75rem) - 上下のセーフエリア。
  // main の下の余白(1.5rem)は -mb-6 で打ち消し、下の空きを 0.75rem だけにする。
  return (
    <div className="-mb-6 flex h-[calc(100svh-6.0625rem-env(safe-area-inset-top)-env(safe-area-inset-bottom))] flex-col gap-3 md:mb-0 md:h-auto">
      <h1 className="text-xl font-bold">何を投稿しますか?</h1>
      <div className="grid min-h-0 flex-1 grid-rows-[repeat(5,minmax(0,1fr))] gap-2 md:auto-rows-auto md:grid-cols-2 md:grid-rows-none md:gap-3 xl:grid-cols-3">
        {doors.map((door) => (
          <Link
            key={door.href}
            href={door.href}
            className={`flex min-h-0 items-center gap-3 overflow-hidden rounded-2xl border border-l-4 border-white/20 glass bg-white/10 px-3 py-2 transition hover:bg-white/16 active:scale-95 ${door.border}`}
          >
            <span className={`flex size-10 shrink-0 items-center justify-center rounded-xl ${door.tile}`}>
              <door.icon className="size-5" aria-hidden />
            </span>
            <span className="min-w-0">
              <span className="block text-base font-semibold leading-tight text-slate-100">{door.title}</span>
              <span className="mt-0.5 line-clamp-2 text-xs leading-snug text-slate-200">{door.body}</span>
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}
