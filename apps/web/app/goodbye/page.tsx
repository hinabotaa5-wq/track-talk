import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "退会しました",
  robots: { index: false },
};

// 退会が終わったあとに飛ぶページ。ログアウト済みなので、ログインなしで見られる場所に置く。
export default function GoodbyePage() {
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">退会しました</h1>
      <p className="text-sm text-slate-300">
        これまで TrackTalk を使っていただき、ありがとうございました。アカウントとデータは削除されました。
      </p>
      <Link
        href="/"
        className="inline-flex w-full items-center justify-center rounded-full bg-blue-600 px-4.5 py-2 text-sm font-semibold text-white shadow-xs transition-all duration-150 hover:bg-blue-500 active:scale-[0.98] active:bg-blue-700"
      >
        ホームへ
      </Link>
    </div>
  );
}
