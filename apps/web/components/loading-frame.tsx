import { PageLoading } from "./page-loading";

export function FeedLoadingFrame({ horizontal = false }: { horizontal?: boolean }) {
  // ホームの横に送る一覧は、横並びの枠の中だと左端に寄るので、画面の真ん中に出す。
  if (horizontal) return <div className="horizontal-feed-track"><PageLoading screen label="投稿を読み込んでいます" /></div>;
  return <PageLoading label="投稿を読み込んでいます" />;
}
export function CompetitionLoadingFrame() {
  return <PageLoading screen label="大会を読み込んでいます" />;
}
export function DetailLoadingFrame() {
  return <PageLoading screen label="投稿を読み込んでいます" />;
}
export function FormLoadingFrame() {
  return <PageLoading screen label="画面を準備しています" />;
}
