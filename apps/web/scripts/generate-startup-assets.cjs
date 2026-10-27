/* eslint-disable @typescript-eslint/no-require-imports -- ロゴ定義の変換にCommonJSのModuleを使用する生成スクリプト。 */
// 実行: node scripts/generate-startup-assets.cjs
// アプリと同じロゴから、PWAとiOSの起動画像を再生成する。起動画像の背景はヘッダーと同じ濃紺。
const fs = require("node:fs");
const path = require("node:path");
const Module = require("node:module");
const ts = require("typescript");
const sharp = require("sharp");
const React = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const root = path.resolve(__dirname, "..");
const screens = require("../lib/startup-screens.json");
const filename = path.join(root, "components/app-icon-mark.tsx");
const component = new Module(filename, module);
component.filename = filename;
component.paths = Module._nodeModulePaths(path.dirname(filename));
component._compile(ts.transpileModule(fs.readFileSync(filename, "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText, filename);
const svg = renderToStaticMarkup(React.createElement(component.exports.TrackLanesMark, {
  xmlns: "http://www.w3.org/2000/svg", width: 512, height: 512,
}));

async function generate() {
  const icons = path.join(root, "public/icons");
  const startup = path.join(root, "public/startup");
  fs.mkdirSync(icons, { recursive: true });
  fs.mkdirSync(startup, { recursive: true });
  for (const size of [192, 512]) {
    await sharp(Buffer.from(svg)).resize(size, size).flatten({ background: "white" })
      .png().toFile(path.join(icons, `tracktalk-${size}.png`));
  }
  for (const [width, height, ratio] of screens) {
    const logo = await sharp(Buffer.from(svg)).resize(192 * ratio, 192 * ratio).png().toBuffer();
    for (const [w, h] of [[width * ratio, height * ratio], [height * ratio, width * ratio]]) {
      await sharp({ create: { width: w, height: h, channels: 3, background: "#16243a" } })
        .composite([{ input: logo, gravity: "centre" }]).png()
        .toFile(path.join(startup, `${w}x${h}.png`));
    }
  }
  console.log(`Generated 2 icons and ${screens.length * 2} startup images.`);
}
generate().catch((error) => { console.error(error); process.exitCode = 1; });
