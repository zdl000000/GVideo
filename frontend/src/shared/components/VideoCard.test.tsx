// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { VideoCard } from "./VideoCard";
import { VideoHero } from "./VideoHero";
import { VideoCover } from "./VideoCover";
import { videoFixture } from "../test/videoFixtures";

afterEach(cleanup);
describe("VideoCard variants", () => {
  it.each(["standard", "editorial", "compact"] as const)("%s 保留同一 Video 的播放与作者链接，并减少 metadata", (variant) => {
    render(<MemoryRouter><VideoCard video={videoFixture(4, { description: "真实描述", title: "很长的中文标题".repeat(10), views_count: 0 })} variant={variant} /></MemoryRouter>);
    expect(screen.getByRole("link", { name: /^播放/ }).getAttribute("href")).toBe("/video/4");
    expect(screen.getByRole("link", { name: "创作者4" }).getAttribute("href")).toBe("/users/14");
    expect(screen.getByText("0 次播放")).toBeTruthy();
    expect(screen.queryByText("真实描述") !== null).toBe(variant === "editorial");
    expect(document.querySelector(".category-label")).toBeNull();
    expect(document.querySelector(".video-stats .lucide-message-circle")).toBeNull();
    expect(document.querySelector("time")?.getAttribute("datetime")).toBe("2026-01-01T00:00:00Z");
  });
  it("ranked 使用传入排名，action 仍独立于视频链接", () => {
    render(<MemoryRouter><VideoCard video={videoFixture()} variant="ranked" rank={37} action={<button>取消收藏</button>} /></MemoryRouter>);
    expect(screen.getByLabelText("第 37 名").textContent).toBe("37");
    expect(screen.getByRole("button").closest("a")).toBeNull();
  });
  it("未迁移消费者仍保留原 category、comments 密度", () => {
    render(<MemoryRouter><VideoCard video={videoFixture()} /></MemoryRouter>);
    expect(screen.getByText("音乐")).toBeTruthy();
    expect(document.querySelector(".video-stats .lucide-message-circle")).toBeTruthy();
    expect(document.querySelector(".gv-video-card")).toBeNull();
  });
});
describe("VideoCover", () => {
  it("空封面只有品牌 fallback，没有失效业务 img", () => {
    const { container } = render(<VideoCover src="" />);
    expect(container.querySelector(".gv-cover-image")).toBeNull();
    expect(container.querySelector(".gv-media-fallback")).toBeTruthy();
  });
  it("默认 lazy、loading 固定占位，404 后移除图片，不再触发循环", () => {
    const { container } = render(<VideoCover src="/missing.jpg" />);
    const img = container.querySelector<HTMLImageElement>(".gv-cover-image")!;
    expect(img.getAttribute("loading")).toBe("lazy");
    expect(container.querySelector(".gv-video-cover--loading")).toBeTruthy();
    fireEvent.error(img);
    expect(container.querySelector(".gv-cover-image")).toBeNull();
    expect(container.querySelector(".gv-video-cover--failed")).toBeTruthy();
  });
  it("新 URL 重置失败状态，成功时移除 fallback", () => {
    const { container, rerender } = render(<VideoCover src="/missing.jpg" />);
    fireEvent.error(container.querySelector(".gv-cover-image")!);
    rerender(<VideoCover src="/valid.jpg" />);
    const img = container.querySelector(".gv-cover-image")!;
    expect(img.getAttribute("src")).toBe("/valid.jpg");
    fireEvent.load(img);
    expect(container.querySelector(".gv-media-fallback")).toBeNull();
    expect(container.querySelector(".gv-video-cover--ready")).toBeTruthy();
  });
  it("Hero eager 加载且与 Card 使用同一个失效处理", () => {
    const { container } = render(<MemoryRouter><VideoHero video={videoFixture(1, { cover_url: "/404.jpg" })} /></MemoryRouter>);
    const img = container.querySelector(".gv-cover-image")!;
    expect(img.getAttribute("loading")).toBe("eager");
    expect(img.getAttribute("fetchpriority")).toBe("high");
    fireEvent.error(img);
    expect(container.querySelector(".gv-cover-image")).toBeNull();
    expect(screen.getByRole("link", { name: "立即观看" }).getAttribute("href")).toBe("/video/1");
  });
});
