import { describe, expect, it } from "vitest";
import geoCountry, { config, countryCode } from "./geo-country";

// TEMPORARY（本体 #565）。言語選択プロンプトを外すときはこのファイルも消す。

describe("アクセス元の国コードを返す端の関数", () => {
  it("国コードだけを返し、キャッシュさせない", async () => {
    const response = geoCountry(new Request("https://example.test/geo-country"), {
      geo: { country: { code: "US" } },
    });
    expect(await response.json()).toEqual({ country: "US" });
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(config.path).toBe("/geo-country");
  });

  it("国が取れないときは null", async () => {
    for (const context of [{}, { geo: null }, { geo: {} }, { geo: { country: null } }, { geo: { country: { code: "" } } }]) {
      const response = geoCountry(new Request("https://example.test/geo-country"), context);
      expect(await response.json()).toEqual({ country: null });
    }
  });

  it("2文字の英字以外は捨て、小文字は大文字にそろえる", () => {
    expect(countryCode({ geo: { country: { code: "us" } } })).toBe("US");
    expect(countryCode({ geo: { country: { code: "USA" } } })).toBeNull();
    expect(countryCode({ geo: { country: { code: "X1" } } })).toBeNull();
    expect(countryCode(undefined)).toBeNull();
  });
});
