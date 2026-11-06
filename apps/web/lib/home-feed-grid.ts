const gap = 12;
const aspectRatio = 4 / 3;

// masonry 導入前と同じ 4:3、12px 間隔、画面幅別の列数でカード幅を決める。
export function createGridLayout(count: number, viewportWidth: number, height: number, columns: number) {
  const tileWidth = Math.min(Math.max(1, (viewportWidth - gap * (columns - 1)) / columns), height * aspectRatio);
  const tileHeight = tileWidth / aspectRatio;
  const rows = Math.max(1, Math.floor((height + gap) / (tileHeight + gap)));
  const pitch = tileWidth + gap;
  const width = Math.max(viewportWidth, Math.ceil(count / rows) * pitch - gap);
  return { count, width, height, tileWidth, tileHeight, rows, pitch };
}

export type GridLayout = ReturnType<typeof createGridLayout>;

export function gridTilePosition(layout: GridLayout, index: number) {
  return {
    left: Math.floor(index / layout.rows) * layout.pitch,
    top: (index % layout.rows) * (layout.tileHeight + gap),
  };
}

// 横位置から直接列を求め、画面付近の投稿だけを描画する。
export function visibleGridIndices(layout: GridLayout, left: number, width: number, overscan = width) {
  const firstColumn = Math.max(0, Math.ceil((left - overscan - layout.tileWidth) / layout.pitch));
  const lastColumn = Math.floor((left + width + overscan) / layout.pitch);
  const start = firstColumn * layout.rows;
  const end = Math.min(layout.count, (lastColumn + 1) * layout.rows);
  return Array.from({ length: Math.max(0, end - start) }, (_, i) => start + i);
}
