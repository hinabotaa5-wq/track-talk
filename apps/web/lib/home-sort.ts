export type HomeSort = "popular" | "new" | "recruitment" | "saved";

export function parseHomeSort(value: string | null | undefined): HomeSort {
  if (value === "new" || value === "recruitment" || value === "saved") return value;
  return "popular";
}

export function homeHref(sort: HomeSort, q: string, page?: number) {
  const params = new URLSearchParams();
  if (sort !== "popular") params.set("sort", sort);
  if (q) params.set("q", q);
  if (page && page > 1) params.set("page", String(page));
  const s = params.toString();
  return s ? `/?${s}` : "/";
}
