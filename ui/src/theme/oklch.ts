/*
 * Token resolution and colour conversion for the design system's tokens
 * (DESIGN-0028 §11, §13). The package's values are oklch, or a reference
 * to another token: `{name}` in its tokens.json, `var(--name)` in its
 * stylesheet. Everything docz checks or generates reads sRGB hex, so the
 * conversion lives here once, shared by contrast.test.ts and
 * scripts/gen-charcoal.ts. It touches no DOM, so a Bun script can
 * import it.
 */

/** A token that cannot be resolved: unknown, cyclic, or not a colour. */
export class TokenError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "TokenError";
  }
}

/** Looks a token up by its bare name (`background`, not `--background`). */
export type TokenSource = (name: string) => string | undefined;

const JSON_REFERENCE = /^\{([A-Za-z0-9-]+)\}$/;
const CSS_REFERENCE = /^var\(\s*--([A-Za-z0-9-]+)\s*\)$/;

/**
 * referenceName returns the token a value points at, for `{name}` and
 * for `var(--name)`, or null when the value is a literal.
 */
export function referenceName(value: string): string | null {
  const trimmed = value.trim();
  return (
    JSON_REFERENCE.exec(trimmed)?.[1] ??
    CSS_REFERENCE.exec(trimmed)?.[1] ??
    null
  );
}

/**
 * resolveToken follows a token through its references to a literal
 * value. An unknown name and a cycle are both errors that name the
 * chain, so a lost mapping never resolves to nothing.
 */
export function resolveToken(name: string, source: TokenSource): string {
  const chain: string[] = [];
  let current = name;
  for (;;) {
    if (chain.includes(current)) {
      throw new TokenError(`token cycle: ${[...chain, current].join(" -> ")}`);
    }
    chain.push(current);
    const value = source(current);
    if (value === undefined) {
      throw new TokenError(
        chain.length === 1
          ? `unknown token: ${current}`
          : `unknown token: ${current} (from ${chain.slice(0, -1).join(" -> ")})`,
      );
    }
    const next = referenceName(value);
    if (next === null) {
      return value.trim();
    }
    current = next;
  }
}

const OKLCH = /^oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)(?:deg)?\s*\)$/;
const HEX_6 = /^#[0-9a-fA-F]{6}$/;
const HEX_3 = /^#([0-9a-fA-F])([0-9a-fA-F])([0-9a-fA-F])$/;

function gammaEncode(linear: number): number {
  const v =
    linear <= 0.0031308
      ? 12.92 * linear
      : 1.055 * Math.pow(linear, 1 / 2.4) - 0.055;
  return Math.min(1, Math.max(0, v));
}

function channelHex(linear: number): string {
  return Math.round(gammaEncode(linear) * 255)
    .toString(16)
    .padStart(2, "0");
}

/**
 * oklchToHex converts `oklch(L C H)` to sRGB hex. A colour outside the
 * sRGB gamut is clipped per channel, which is what a browser paints.
 */
export function oklchToHex(value: string): string {
  const match = OKLCH.exec(value.trim());
  if (match === null) {
    throw new TokenError(`not an oklch() colour: ${value}`);
  }
  const lightness = Number(match[1]) / (match[2] === "%" ? 100 : 1);
  const chroma = Number(match[3]);
  const hue = (Number(match[4]) * Math.PI) / 180;
  const a = chroma * Math.cos(hue);
  const b = chroma * Math.sin(hue);

  // OKLab to linear sRGB (Björn Ottosson's matrices).
  const l = Math.pow(lightness + 0.3963377774 * a + 0.2158037573 * b, 3);
  const m = Math.pow(lightness - 0.1055613458 * a - 0.0638541728 * b, 3);
  const s = Math.pow(lightness - 0.0894841775 * a - 1.291485548 * b, 3);

  const red = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const green = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const blue = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;

  return `#${channelHex(red)}${channelHex(green)}${channelHex(blue)}`;
}

/** colorToHex turns a literal colour (hex or oklch) into `#rrggbb`. */
export function colorToHex(value: string): string {
  const trimmed = value.trim();
  if (HEX_6.test(trimmed)) {
    return trimmed.toLowerCase();
  }
  const short = HEX_3.exec(trimmed);
  if (short !== null) {
    return `#${short
      .slice(1)
      .map((c) => c + c)
      .join("")}`.toLowerCase();
  }
  return oklchToHex(trimmed);
}

/** tokenHex resolves a token and converts it: the usual call. */
export function tokenHex(name: string, source: TokenSource): string {
  return colorToHex(resolveToken(name, source));
}
