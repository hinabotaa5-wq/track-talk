const gap = 12;
const aspectRatio = 4 / 3;
// ホーム投稿一覧の段数は常に3に固定する。帯の有無などで表示領域の高さが変わっても、
// 段数の決め方を変えない（帯への分岐で直さない）。高さが変わったら札の大きさで吸収する。
const fixedRows = 3;

// 使える高さを3段で均等に使い、右端は横スクロールで表示する。
export function createGridLayout(count: number, viewportWidth: number, height: number) {
  const rows = fixedRows;
  const tileHeight = Math.max(1, (height - gap * (rows - 1)) / rows);
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
