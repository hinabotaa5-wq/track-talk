import { DetailLoadingFrame } from "@/components/loading-frame";

// 画面を開くたびに、サーバーが答えるまで前の画面のまま固まって見えないよう、すぐ骨組みを出す。
export default function Loading() {
  return <DetailLoadingFrame />;
}
