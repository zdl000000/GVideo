import { describe, expect, it } from "vitest";

// Node runs this test; keep its file-reading type local instead of adding Node
// globals to the browser app. Vitest intentionally stubs CSS module imports.
const fsModule = "node:fs";
const { readFileSync } = await import(/* @vite-ignore */ fsModule) as { readFileSync(path: URL, encoding: "utf8"): string };
const stylesheet = (name: string) => readFileSync(new URL(`../../styles/${name}.css`, import.meta.url), "utf8");
const tokensCss = stylesheet("tokens");
const watchCss = stylesheet("watch");
const playbackCss = stylesheet("watch-playback");
const studioCss = stylesheet("studio-workspace");
const publishCss = stylesheet("publish");
const notificationsCss = stylesheet("notifications");
const governanceCss = stylesheet("governance");
const baseCss = stylesheet("base");

type RGB = [number, number, number];
type Tokens = Record<string, string>;

function declarations(source: string): Tokens {
  return Object.fromEntries([...source.matchAll(/(--[\w-]+)\s*:\s*([^;{}]+);/g)].map((match) => [match[1], match[2].trim()]));
}

function resolve(value: string, tokens: Tokens): string {
  const variable = /^var\((--[\w-]+)\)$/.exec(value);
  return variable ? resolve(tokens[variable[1]], tokens) : value;
}

function rgb(hex: string): RGB {
  if (hex === "white") hex = "#fff";
  if (hex === "black") hex = "#000";
  if (!/^#(?:[\da-f]{3}|[\da-f]{6})$/i.test(hex)) throw new Error(`Unsupported audit color: ${hex}`);
  const digits = hex.slice(1).length === 3 ? [...hex.slice(1)].map((digit) => digit + digit).join("") : hex.slice(1);
  return [0, 2, 4].map((offset) => parseInt(digits.slice(offset, offset + 2), 16) / 255) as RGB;
}

// WCAG 2.2: linearized sRGB luminance; compare the unrounded ratio to 4.5 / 3.
// https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html
function luminance(color: RGB): number {
  const channels = color.map((value) => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4);
  return .2126 * channels[0] + .7152 * channels[1] + .0722 * channels[2];
}

function contrast(foreground: RGB, background: RGB): number {
  const values = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (values[0] + .05) / (values[1] + .05);
}

function composite(foreground: RGB, alpha: number, background: RGB): RGB {
  return foreground.map((channel, index) => channel * alpha + background[index] * (1 - alpha)) as RGB;
}

function rgbaOver(value: string, background: RGB): RGB {
  const match = /^rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([.\d]+)\s*\)$/.exec(value);
  if (!match) throw new Error(`Unsupported audit tint: ${value}`);
  return composite([Number(match[1]) / 255, Number(match[2]) / 255, Number(match[3]) / 255], Number(match[4]), background);
}

const blocks = [...tokensCss.matchAll(/:root(?:\[data-theme="light"\])?\s*\{([^}]+)\}/g)];
const dark = declarations(blocks[0][1]);
const light = { ...dark, ...declarations(blocks[1][1]) };
const profiles = [
  { name: "Dark root", tokens: dark },
  { name: "Light root / Auth", tokens: light },
  ...[
    ["Light WATCH Discovery", watchCss], ["Light Playback / Creator", playbackCss],
    ["Light Studio / Content", studioCss], ["Light Publish", publishCss],
    ["Light Notifications", notificationsCss], ["Light Governance", governanceCss]
  ].map(([name, css]) => {
    const scoped = [...css.matchAll(/:root\[data-theme="light"\][^{]*\{([^}]+)\}/g)]
      .map((match) => declarations(match[1])).find((values) => values["--gv-bg"]);
    if (!scoped) throw new Error(`Missing actual light surface declarations: ${name}`);
    return { name, tokens: { ...light, ...scoped } };
  })
];
const surfaces = ["--gv-bg", "--gv-bg-elevated", "--gv-surface", "--gv-surface-2"];
const foregrounds = ["--gv-text", "--gv-text-2", "--gv-text-3", "--gv-brand-text", "--gv-cyan-text", "--gv-success-text", "--gv-danger-text", "--gv-warning-text", "--gv-info-text"];

function assertRatio(foreground: string, background: RGB, tokens: Tokens, label: string, minimum: number) {
  const ratio = contrast(rgb(resolve(tokens[foreground], tokens)), background);
  expect(ratio, `${label}: ${ratio.toFixed(4)}:1`).toBeGreaterThanOrEqual(minimum);
}

describe("actual GVideo foreground contrast", () => {
  it("calibrates black / white to 21:1 and equal colors to 1:1", () => {
    expect(contrast(rgb("#000"), rgb("#fff"))).toBe(21);
    expect(contrast(rgb("#ff2f62"), rgb("#ff2f62"))).toBe(1);
  });

  it.each(profiles)("$name: body, secondary and semantic text meet 4.5:1", ({ name, tokens }) => {
    for (const surface of surfaces) for (const foreground of foregrounds) {
      assertRatio(foreground, rgb(tokens[surface]), tokens, `${name} ${foreground} / ${surface}`, 4.5);
    }
  });

  it.each(profiles)("$name: focus meets 3:1 on actual canvas and surfaces", ({ name, tokens }) => {
    for (const surface of surfaces) assertRatio("--gv-focus", rgb(tokens[surface]), tokens, `${name} focus / ${surface}`, 3);
  });

  it.each(profiles)("$name: status text meets 4.5:1 on its actual translucent tint", ({ name, tokens }) => {
    // These tints are used on page canvases, rails and content surfaces, not on
    // already tinted secondary fills. Alpha is composited before measuring.
    for (const surface of surfaces.filter((value) => value !== "--gv-surface-2")) {
      const background = rgb(tokens[surface]);
      for (const semantic of ["brand", "cyan", "danger", "warning"]) {
        const foreground = `--gv-${semantic}-text`;
        const tinted = rgbaOver(tokens[`--gv-${semantic}-soft`], background);
        assertRatio(foreground, tinted, tokens, `${name} ${foreground} / tinted ${surface}`, 4.5);
      }
      // The 8% success / info tint belongs only to Governance status badges.
      // Other consumers use their opaque text token on the base surface.
      if (name === "Dark root" || name === "Light Governance") {
        for (const semantic of ["success", "info"]) {
          const foreground = `--gv-${semantic}-text`;
          const tinted = composite(rgb(tokens[foreground]), .08, background);
          assertRatio(foreground, tinted, tokens, `${name} ${foreground} / status ${surface}`, 4.5);
        }
      }
    }
  });

  it.each([dark, light])("primary CTA / brand badge text meets 4.5:1 at rest and hover", (tokens) => {
    for (const selector of ["gv-button--primary", "primary-button", "danger-button", "gv-badge--brand"]) {
      const rule = new RegExp(`\\.${selector}\\s*\\{([^}]+)\\}`).exec(baseCss)?.[1];
      if (!rule) throw new Error(`Missing button audit rule: ${selector}`);
      const foreground = /(?:^|;)\s*color:\s*([^;]+);/.exec(rule)?.[1];
      const background = /(?:^|;)\s*background:\s*([^;]+);/.exec(rule)?.[1];
      if (!foreground || !background) throw new Error(`Missing foreground / background: ${selector}`);
      const text = rgb(resolve(foreground.trim(), tokens));
      const fill = rgb(resolve(background.trim(), tokens));
      expect(contrast(text, fill), `${selector} at rest`).toBeGreaterThanOrEqual(4.5);
      expect(contrast(text, rgb(tokens["--gv-brand-hover"])), `${selector} hover fill`).toBeGreaterThanOrEqual(4.5);
    }
  });
});
