const jst = "Asia/Tokyo";

export function formatDateTime(iso: string) {
  return new Intl.DateTimeFormat("ja-JP", {
    timeZone: jst,
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(iso));
}

export function formatDate(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso.slice(0, 10);
  }
  return new Intl.DateTimeFormat("ja-JP", {
    timeZone: jst,
    dateStyle: "medium",
  }).format(d);
}

export function scopeLabel(scope: string) {
  return scope === "international" ? "国際大会" : "国内大会";
}
