export type MosaicItem = { ratio: number; footer: number };
export type MosaicTile = { x: number; y: number; width: number; height: number };

// 列やページを作らず、既存のカードの端から次の空きを探す。
// 入力順を維持し、後の投稿が先の投稿より左へ戻らないようにする。
export function packMosaic(items: MosaicItem[], viewportWidth: number, height: number, gap = 12) {
  const landscapeWidth = Math.max(176, Math.min(308, viewportWidth * 0.605));
  const portraitHeight = Math.min(264, height * 0.55);
  const tiles: MosaicTile[] = [];
  let lastX = 0;

  for (const item of items) {
    const ratio = Number.isFinite(item.ratio) && item.ratio > 0 ? item.ratio : 4 / 3;
    const footer = Math.max(0, item.footer);
    const preferredWidth = ratio < 1 ? portraitHeight * ratio + 2 : landscapeWidth;
    const width = Math.min(preferredWidth, (height - footer - 2) * ratio + 2);
    const xs = [...new Set([lastX, ...tiles.map((tile) => tile.x + tile.width + gap)])]
      .filter((x) => x >= lastX).sort((a, b) => a - b);
    const ys = [...new Set([0, ...tiles.map((tile) => tile.y + tile.height + gap)])].sort((a, b) => a - b);
    let next: MosaicTile | undefined;
    for (const x of xs) {
      for (const y of ys) {
        // 画面の下だけでなく、下にあるカードまでの空きにも縮めて収める。
        const bottoms = [height, ...tiles.filter((tile) => tile.y > y).map((tile) => tile.y - gap)];
        const widths = [...new Set(bottoms.map((bottom) => Math.min(width, (bottom - y - footer - 2) * ratio + 2)))]
          .filter((candidate) => candidate >= width * (ratio < 1 ? 0.6 : 0.78)).sort((a, b) => b - a);
        for (const fittedWidth of widths) {
          const tileHeight = (fittedWidth - 2) / ratio + footer + 2;
          const overlaps = tiles.some((tile) => x < tile.x + tile.width + gap - 0.1 && x + fittedWidth + gap > tile.x + 0.1
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
    if (!next) next = { x: Math.max(0, ...tiles.map((tile) => tile.x + tile.width + gap)), y: 0, width, height: (width - 2) / ratio + footer + 2 };
    tiles.push(next);
    lastX = next.x;
  }
  return { tiles, width: Math.max(viewportWidth, ...tiles.map((tile) => tile.x + tile.width)), height };
}
