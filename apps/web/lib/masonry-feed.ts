export type MosaicItem = { ratio: number; footer: number; compact?: boolean };
export type MosaicTile = { x: number; y: number; width: number; height: number };

// 列やページを作らず、既存のカードの端から次の空きを探す。
// 投稿順に配置し、後の投稿も手前に残った空きへ詰める。
// placed は前のブロックの右端付近のカードで、その間に残った空きにも詰める。
function packBlock(items: MosaicItem[], viewportWidth: number, height: number, gap = 12, placed: MosaicTile[] = []) {
  const landscapeWidth = Math.max(176, Math.min(308, viewportWidth * 0.605));
  // 縦動画は走りの動きが見やすい大きさにする。文章だけの投稿は少し小さく、写真・動画を優先する。
  const portraitHeight = Math.min(280, height * 0.56);
  const tiles: MosaicTile[] = [];

  for (const item of items) {
    const ratio = Number.isFinite(item.ratio) && item.ratio > 0 ? item.ratio : 4 / 3;
    const footer = Math.max(0, item.footer);
    const preferredWidth = ratio < 1 ? portraitHeight * ratio + 2 : landscapeWidth * (item.compact ? 0.78 : 1);
    const width = Math.min(preferredWidth, (height - footer - 2) * ratio + 2);
    const all = [...placed, ...tiles];
    const xs = [...new Set([...(placed.length ? [] : [0]), ...all.map((tile) => tile.x + tile.width + gap)])]
      .sort((a, b) => a - b);
    const ys = [...new Set([0, ...all.map((tile) => tile.y + tile.height + gap)])].sort((a, b) => a - b);
    let next: MosaicTile | undefined;
    for (const x of xs) {
      for (const y of ys) {
        // 画面の下だけでなく、下にあるカードまでの空きにも縮めて収める。
        const bottoms = [height, ...all.filter((tile) => tile.y > y).map((tile) => tile.y - gap)];
        const widths = [...new Set(bottoms.map((bottom) => Math.min(width, (bottom - y - footer - 2) * ratio + 2)))]
          .filter((candidate) => candidate >= width * (ratio < 1 ? 0.6 : 0.78)).sort((a, b) => b - a);
        for (const fittedWidth of widths) {
          const tileHeight = (fittedWidth - 2) / ratio + footer + 2;
          const overlaps = all.some((tile) => x < tile.x + tile.width + gap - 0.1 && x + fittedWidth + gap > tile.x + 0.1
            && y < tile.y + tile.height + gap - 0.1 && y + tileHeight + gap > tile.y + 0.1);
          if (!overlaps) {
            next = { x, y, width: fittedWidth, height: tileHeight };
            break;
          }
        }
        if (next) break;
      }
      if (next) break;
    }
    // 収まらない投稿は右へ続け、画面端で自然に見切れさせる。
    if (!next) next = { x: Math.max(0, ...all.map((tile) => tile.x + tile.width + gap)), y: 0, width, height: (width - 2) / ratio + footer + 2 };
    tiles.push(next);
  }
  return { tiles, width: Math.max(viewportWidth, ...[...placed, ...tiles].map((tile) => tile.x + tile.width)), height };
}

// 既存の詰め方を12件ごとにまとめ、過去の全カードを探索しない。
// 次のブロックは前のブロックの右端付近のカードだけを見て、その間の空きから詰める。
const mosaicBlockSize = 12;
type Block = { key: string; tiles: MosaicTile[] };
export function createMosaicPacker() {
  let blocks: Block[] = [];
  let dimensions = "";
  return (items: MosaicItem[], viewportWidth: number, height: number, gap = 12) => {
    const nextDimensions = `${viewportWidth}:${height}:${gap}`;
    if (dimensions !== nextDimensions) { blocks = []; dimensions = nextDimensions; }
    const tiles: MosaicTile[] = [];
    const ranges: { start: number; end: number; left: number; right: number }[] = [];
    // 前のブロックが作り直されたら、それに続くブロックも作り直す。
    let reuse = true;
    for (let start = 0; start < items.length; start += mosaicBlockSize) {
      const slice = items.slice(start, start + mosaicBlockSize);
      const key = slice.map((item) => `${item.ratio}:${item.footer}:${Boolean(item.compact)}`).join(",");
      const index = start / mosaicBlockSize;
      let block = blocks[index];
      if (!reuse || block?.key !== key) {
        reuse = false;
        // 前のブロックの左端より右に掛かるカードだけが、次の配置に関わる。
        const bound = ranges.at(-1)?.left ?? 0;
        const placed: MosaicTile[] = [];
        for (let b = ranges.length - 1; b >= 0 && ranges[b].right > bound; b--) {
          for (let i = ranges[b].start; i < ranges[b].end; i++) {
            if (tiles[i].x + tiles[i].width > bound) placed.push(tiles[i]);
          }
        }
        block = { key, tiles: packBlock(slice, viewportWidth, height, gap, placed).tiles };
        blocks[index] = block;
      }
      tiles.push(...block.tiles);
      const right = Math.max(ranges.at(-1)?.right ?? 0, ...block.tiles.map((tile) => tile.x + tile.width));
      ranges.push({ start, end: tiles.length, left: Math.min(...block.tiles.map((tile) => tile.x)), right });
    }
    blocks.length = Math.ceil(items.length / mosaicBlockSize);
    return { tiles, width: Math.max(viewportWidth, ranges.at(-1)?.right ?? 0), height, ranges };
  };
}
export type MosaicLayout = ReturnType<ReturnType<typeof createMosaicPacker>>;
export function packMosaic(items: MosaicItem[], viewportWidth: number, height: number, gap = 12) {
  return createMosaicPacker()(items, viewportWidth, height, gap);
}

// スクロール時には全投稿を走査せず、座標順のブロックから表示対象を絞る。
export function visibleMosaicIndices(layout: MosaicLayout, left: number, width: number, overscan = width) {
  const min = Math.max(0, left - overscan), max = left + width + overscan;
  let lo = 0, hi = layout.ranges.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if (layout.ranges[mid].right < min) lo = mid + 1;
    else hi = mid;
  }
  const indices: number[] = [];
  for (let b = lo; b < layout.ranges.length && layout.ranges[b].left <= max; b++) {
    const block = layout.ranges[b];
    for (let i = block.start; i < block.end; i++) {
      const tile = layout.tiles[i];
      if (tile.x <= max && tile.x + tile.width >= min) indices.push(i);
    }
  }
  return indices;
}
