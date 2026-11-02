import { PageLoading } from "./page-loading";

export function FeedLoadingFrame({ horizontal = false }: { horizontal?: boolean }) {
  return <div className={horizontal ? "horizontal-feed-track" : undefined}><PageLoading label="投稿を読み込んでいます" /></div>;
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
