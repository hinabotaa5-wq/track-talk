// Node 24: node scripts/measure-home-feed.mjs
// 同じ混合投稿・スマホ幅で配置計算だけを比較する。通信・描画時間は含まない。
import { packMosaic } from "../lib/masonry-feed.ts";

const results = [];
for (const count of [12, 120, 600]) {
  const items = Array.from({ length: count }, (_, i) => ({
    ratio: [9 / 16, 16 / 9, 1, 4 / 3][i % 4], footer: 0, compact: i % 5 === 0,
  }));
  const started = performance.now();
  const layout = packMosaic(items, 390, 560);
  results.push({ posts: count, layoutMs: +(performance.now() - started).toFixed(2), tiles: layout.tiles.length });
}
console.table(results);
