import { describe, expect, it } from "vitest";

const sharedModules = import.meta.glob(["./**/*.ts", "./**/*.tsx", "!./**/*.test.ts", "!./**/*.test.tsx"], {
  eager: true,
  query: "?raw",
  import: "default"
}) as Record<string, string>;

function importSpecifiers(source: string): string[] {
  const specifiers: string[] = [];
  const pattern = /\b(?:import|export)\s+(?:type\s+)?(?:[^"']*?\s+from\s+)?["']([^"']+)["']/g;
  for (const match of source.matchAll(pattern)) specifiers.push(match[1]);
  return specifiers;
}

function forbiddenSharedImport(specifier: string): boolean {
  if (!specifier.startsWith(".")) return false;
  const segments: string[] = [];
  for (const segment of specifier.split("/")) {
    if (segment === "." || segment === "") continue;
    if (segment === "..") segments.pop();
    else segments.push(segment);
  }
  return segments[0] === "app" || segments[0] === "features";
}

describe("shared architecture boundary", () => {
  it("does not import app or feature modules", () => {
    const violations = Object.entries(sharedModules).flatMap(([path, source]) =>
      importSpecifiers(source)
        .filter(forbiddenSharedImport)
        .map((specifier) => `${path}: ${specifier}`)
    );

    expect(violations, "shared modules must remain independent from app and features").toEqual([]);
  });
});