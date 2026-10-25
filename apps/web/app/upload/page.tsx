"use client";

import { postOptions } from "@/lib/post-options";
import { MainTabLink } from "@/components/main-tab-link";
import { useAuth } from "@/components/auth-provider";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { UploadOpening } from "./upload-opening";

export default function UploadPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/upload");
  }, [loading, session, router]);

  if (!session) return <UploadOpening />;

  // ホームの「＋」から開く詳細画面。下のメニューは出さないので、その分まで5つの入口で埋める。
  // dvh だと、iPhone の Safari で画面を開いた直後に下のツールバーが出てきた分だけ高さが縮み、
  // 5つの入口が上にギュッと詰まる。ツールバーが出ている時の高さ(svh)で固定して動かさない。
  // 高さ = 画面 - ヘッダー(3.8125rem) - 上の余白(1.5rem) - 下の余白(0.75rem) - 上下のセーフエリア。
  // main の下の余白(1.5rem)は -mb-6 で打ち消し、下の空きを 0.75rem だけにする。
  return (
    <div className="-mb-6 flex h-[calc(100svh-6.0625rem-env(safe-area-inset-top)-env(safe-area-inset-bottom))] flex-col gap-3 md:mb-0 md:h-auto">
      <h1 className="text-xl font-bold">何を投稿しますか?</h1>
      <div className="grid min-h-0 flex-1 grid-rows-[repeat(5,minmax(0,1fr))] gap-2 md:auto-rows-auto md:grid-cols-2 md:grid-rows-none md:gap-3 xl:grid-cols-3">
        {postOptions.map((door) => (
          <MainTabLink
            key={door.href}
            href={door.href}
            className={`flex min-h-0 items-center gap-3 overflow-hidden rounded-2xl border border-l-4 border-white/20 glass bg-white/10 px-3 py-2 transition hover:bg-white/16 active:scale-[0.98] ${door.border}`}
          >
            <span className={`flex size-10 shrink-0 items-center justify-center rounded-xl ${door.tile}`}>
              <door.icon className="size-5" aria-hidden />
            </span>
            <span className="min-w-0">
              <span className="block text-base font-semibold leading-tight text-slate-100">{door.title}</span>
              <span className="mt-0.5 line-clamp-2 text-xs leading-snug text-slate-200">{door.body}</span>
            </span>
          </MainTabLink>
        ))}
      </div>
    </div>
  );
}
