import { parseXPostUrl } from "@/lib/x-url";
import { parseInstagramUrl } from "@/lib/instagram";
import { isYouTubeUrl } from "@/lib/validators";

export type VideoUrlKind = "youtube" | "instagram" | "x";

// 対応するサービスの投稿URLを見分ける。
export function detectVideoUrl(value: string): VideoUrlKind | null {
  if (isYouTubeUrl(value)) return "youtube";
  if (parseInstagramUrl(value)) return "instagram";
  if (parseXPostUrl(value)) return "x";
  return null;
}
