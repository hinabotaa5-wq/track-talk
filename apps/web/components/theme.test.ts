import { readFileSync } from "node:fs";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";

const css = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8");
const colors = Object.fromEntries(
  Array.from(css.matchAll(/--([\w-]+):\s*(#[\da-f]{6});/g), ([, name, color]) => [name, color]),
);

function luminance(hex: string) {
  const rgb = hex.slice(1).match(/../g)!.map((value) => {
    const channel = parseInt(value, 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return rgb[0] * 0.2126 + rgb[1] * 0.7152 + rgb[2] * 0.0722;
}

function contrast(foreground: string, background: string) {
  const values = [luminance(colors[foreground]), luminance(colors[background])].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

describe("blue-gray theme", () => {
  it("uses dark blue-gray surfaces and dark native controls", () => {
    expect(css).toContain("color-scheme: dark");
    for (const name of ["background", "surface", "surface-raised", "field"]) {
      expect(luminance(colors[name])).toBeLessThan(0.1);
      expect(parseInt(colors[name].slice(5, 7), 16)).toBeGreaterThan(parseInt(colors[name].slice(1, 3), 16));
    }
  });

  it.each([
    ["foreground", "background"],
    ["foreground", "surface"],
    ["foreground", "surface-raised"],
    ["muted", "surface"],
    ["muted", "surface-raised"],
    ["muted", "field"],
    ["muted", "accent-soft"],
    ["accent", "surface"],
    ["accent", "accent-soft"],
    ["accent-foreground", "accent"],
    ["accent-foreground", "accent-hover"],
    ["danger", "surface"],
    ["danger", "danger-soft"],
    ["danger-foreground", "danger-strong"],
    ["danger-foreground", "danger-hover"],
    ["like", "like-soft"],
    ["success", "background"],
  ])("keeps %s readable on %s at WCAG AA contrast", (foreground, background) => {
    expect(contrast(foreground, background)).toBeGreaterThanOrEqual(4.5);
  });

  it("keeps input borders and focus indicators distinct", () => {
    expect(contrast("input-border", "field")).toBeGreaterThanOrEqual(3);
    expect(contrast("accent", "surface")).toBeGreaterThanOrEqual(3);
    expect(css).toContain(":focus-visible");
  });

  it.each([
    ["primary", "bg-accent", "text-accent-foreground"],
    ["secondary", "bg-surface", "text-foreground"],
    ["danger", "bg-danger-strong", "text-danger-foreground"],
  ] as const)("uses matching colors for %s buttons", (variant, background, foreground) => {
    const html = renderToStaticMarkup(createElement(Button, { variant }, "送信"));
    expect(html).toContain(background);
    expect(html).toContain(foreground);
    expect(html).not.toContain("bg-white");
  });

  it("uses recessed surfaces for input and textarea fields", () => {
    for (const html of [renderToStaticMarkup(createElement(Input)), renderToStaticMarkup(createElement(Textarea))]) {
      expect(html).toContain("bg-field");
      expect(html).toContain("text-foreground");
      expect(html).toContain("placeholder:text-muted");
      expect(html).toContain("focus:ring-accent/20");
    }
  });
});
