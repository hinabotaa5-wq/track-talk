import { L } from "./i18n";
import { FileImage, Handshake, Link2, Users, Video } from "lucide-react";

// 投稿の入口と利用ガイドで名称・説明・アイコンを共用する。
export const postOptions = [
  { href: "/posts/new", get title() { return L("文章・写真"); }, get body() { return L("文章だけ、写真だけ、両方でも投稿できます。"); }, icon: FileImage, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/url", get title() { return L("動画URL"); }, get body() { return L("YouTube、Instagram、XのURLを貼ってスレッドを立てます。"); }, icon: Link2, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/video", get title() { return L("自分の動画"); }, get body() { return L("自分の走った動画を投稿します。"); }, icon: Video, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/recruit", get title() { return L("クラブチームの募集"); }, get body() { return L("チーム名や活動場所を書いて、メンバーを募ります。"); }, icon: Users, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
  { href: "/posts/practice", get title() { return L("練習相手募集"); }, get body() { return L("練習場所と日時を書いて、一緒に練習する相手を見つけます。"); }, icon: Handshake, border: "border-l-blue-400", tile: "bg-blue-500/15 text-blue-300" },
] as const;
