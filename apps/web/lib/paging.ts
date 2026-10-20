// fetchAllPages は、1ページずつ返る一覧を最後のページまで集める。
// API は per_page を50件までに切り詰めるので、1回だけ読むと51件目以降が見えなくなる。
// total に届いたとき、空のページが返ったとき、maxPages に達したときに止める。
export async function fetchAllPages<T>(
  fetchPage: (page: number) => Promise<{ items: T[]; total: number }>,
  maxPages = 20,
): Promise<{ items: T[]; total: number }> {
  const items: T[] = [];
  let total = 0;
  for (let page = 1; page <= maxPages; page++) {
    const got = await fetchPage(page);
    total = got.total;
    if (got.items.length === 0) break;
    items.push(...got.items);
    if (items.length >= total) break;
  }
  return { items, total };
}
