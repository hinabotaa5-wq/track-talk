import { apiFetch } from "./api";

export type MeetResources = {
  meet_key: string;
  official_url: string;
  timetable_url: string;
  youtube_video_id: string;
  major_slug?: string;
  major_year?: number;
  competition_id?: string;
};

export function meetExternalURL(raw: string) {
  try {
    const url = new URL(raw.trim());
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password ? url.href : "";
  } catch { return ""; }
}

// 既存の実況本文の公式URLもリンクにする。リンク先以外の本文は残す。
export function splitMeetBody(body: string) {
  let official = "";
  const text = body.split("\n").filter((line) => {
    const match = line.trim().match(/^公式サイト:\s*(.+)$/);
    const url = match ? meetExternalURL(match[1]) : "";
    if (!url) return true;
    official = url;
    return false;
  }).join("\n").trim();
  return { text, official_url: official };
}

export async function fetchMeetResources(): Promise<MeetResources[]> {
  try { return (await apiFetch<{ meets: MeetResources[] }>("/v1/meet-resources")).meets ?? []; }
  catch { return []; }
}
