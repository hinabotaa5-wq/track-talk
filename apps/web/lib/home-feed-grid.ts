const gap = 12;
const aspectRatio = 4 / 3;
const contentGutters = 32;

// 画面幅から段数の目安を決め、使える高さを均等に使う。右端は横スクロールで表示する。
export function createGridLayout(count: number, viewportWidth: number, height: number, columns: number) {
  const targetWidth = Math.max(1, (viewportWidth - contentGutters - gap * (columns - 1)) / columns);
  const rows = Math.max(1, Math.floor((height + gap) / (targetWidth / aspectRatio + gap)));
  const tileHeight = (height - gap * (rows - 1)) / rows;
  const tileWidth = tileHeight * aspectRatio;
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
