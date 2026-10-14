export type HomeSort = "popular" | "new" | "recruitment" | "review" | "saved";
export type ProductKind = "shoe" | "spike";

export function parseHomeSort(value: string | null | undefined): HomeSort {
  if (value === "new" || value === "recruitment" || value === "review" || value === "saved") return value;
  return "popular";
}

export function parseProductKind(value: string | null | undefined): ProductKind {
  return value === "spike" ? "spike" : "shoe";
}

export function homeHref(sort: HomeSort, q: string, page?: number, type?: ProductKind) {
  const params = new URLSearchParams();
  if (sort !== "popular") params.set("sort", sort);
  if (sort === "review" && type === "spike") params.set("type", type);
  if (q) params.set("q", q);
  if (page && page > 1) params.set("page", String(page));
  const s = params.toString();
  return s ? `/?${s}` : "/";
}
