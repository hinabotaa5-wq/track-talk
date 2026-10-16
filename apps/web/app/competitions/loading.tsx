import { CompetitionLoadingFrame } from "@/components/loading-frame";

export default function CompetitionsLoading() {
  // 読み込み中に1画面に収まると、iPhone の Safari で下部バーが動く(page.tsx と同じ理由)。
  return (
    <div className="min-h-svh md:min-h-0">
      <CompetitionLoadingFrame />
    </div>
  );
}
