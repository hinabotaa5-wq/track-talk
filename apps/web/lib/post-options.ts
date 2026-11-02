import { FileImage, Handshake, Link2, Users, Video } from "lucide-react";

// 投稿の入口と利用ガイドで名称・説明・アイコンを共用する。
export const postOptions = [
  { href: "/posts/new", title: "文章・写真", body: "文章だけ、写真だけ、両方でも投稿できます。", icon: FileImage, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/url", title: "動画URL", body: "YouTube、Instagram、XのURLを貼ってスレッドを立てます。", icon: Link2, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/video", title: "自分の動画", body: "自分の走った動画を投稿します。", icon: Video, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/recruit", title: "クラブチームの募集", body: "チーム名や活動場所を書いて、メンバーを募ります。", icon: Users, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/practice", title: "練習相手募集", body: "練習場所と日時を書いて、一緒に練習する相手を見つけます。", icon: Handshake, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
] as const;
