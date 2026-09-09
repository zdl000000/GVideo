// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { RouteErrorBoundary } from "./RouteErrorBoundary";

function FailedChunk(): never {
  throw new Error("Failed to fetch dynamically imported module");
}

describe("RouteErrorBoundary", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("动态页面资源加载失败时显示可恢复提示", () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    render(<RouteErrorBoundary><FailedChunk /></RouteErrorBoundary>);

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "页面加载失败" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "重新加载页面" })).toBeTruthy();
  });
});
