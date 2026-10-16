import { parseInstagramUrl } from "@/lib/instagram";
import { isYouTubeUrl } from "@/lib/validators";

export type VideoUrlKind = "youtube" | "instagram";

// 貼られた URL が YouTube か Instagram かを見分ける。どちらでもなければ null。
export function detectVideoUrl(value: string): VideoUrlKind | null {
  if (isYouTubeUrl(value)) return "youtube";
  if (parseInstagramUrl(value)) return "instagram";
  return null;
}
