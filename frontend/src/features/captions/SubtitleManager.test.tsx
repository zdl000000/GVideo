// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { api } from "../../shared/api/client";
import { videoFixture } from "../../shared/test/videoFixtures";
import { SubtitleManager } from "./SubtitleManager";
vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { uploadSubtitle: vi.fn(), setDefaultSubtitle: vi.fn(), deleteSubtitle: vi.fn() } }));
const tracks = [{ id: 1, language: "zh-CN", label: "中文", url: "/one.vtt", is_default: true }, { id: 2, language: "en", label: "English", url: "/two.vtt", is_default: false }];
beforeEach(() => { vi.clearAllMocks(); vi.stubGlobal("confirm", vi.fn(() => true)); });
afterEach(cleanup); afterAll(() => vi.resetModules());
describe("SubtitleManager", () => {
  it("empty state is quiet and upload keeps language/label/file FormData", async () => {
    const changed = vi.fn(); vi.mocked(api.uploadSubtitle).mockResolvedValue(tracks[0]);
    render(<SubtitleManager video={videoFixture()} onTracksChanged={changed} workspace />);
    expect(screen.getByText("暂无字幕轨道，可以在下方上传第一条字幕。")).toBeTruthy();
    const file = new File(["WEBVTT"], "test.vtt", { type: "text/vtt" });
    fireEvent.change(screen.getByLabelText("字幕文件"), { target: { files: [file] } }); fireEvent.submit(document.querySelector(".subtitle-upload-form")!);
    expect(await screen.findByRole("status")).toBeTruthy(); expect(changed).toHaveBeenCalledWith([tracks[0]]);
    const form = vi.mocked(api.uploadSubtitle).mock.calls[0][1] as FormData;
    expect(form.get("subtitle")).toBe(file); expect(form.get("subtitle_language")).toBe("zh-CN"); expect(form.get("subtitle_label")).toBe("中文");
  });
  it("track list exposes a default badge and changes default through the existing API", async () => {
    const changed = vi.fn(); vi.mocked(api.setDefaultSubtitle).mockResolvedValue(tracks);
    render(<SubtitleManager video={videoFixture(3, { subtitle_tracks: tracks })} onTracksChanged={changed} workspace />);
    expect(screen.getByText("默认")).toBeTruthy(); expect(screen.getByText("2 条轨道")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "设为默认" }));
    await screen.findByRole("status"); expect(api.setDefaultSubtitle).toHaveBeenCalledWith(3, 2); expect(changed).toHaveBeenCalledWith(tracks);
  });
  it("delete retains confirmation and returns the server track list", async () => {
    const changed = vi.fn(); vi.mocked(api.deleteSubtitle).mockResolvedValue([tracks[1]]);
    render(<SubtitleManager video={videoFixture(3, { subtitle_tracks: tracks })} onTracksChanged={changed} workspace />);
    fireEvent.click(screen.getAllByRole("button", { name: "删除" })[0]);
    await screen.findByRole("status"); expect(window.confirm).toHaveBeenCalled(); expect(api.deleteSubtitle).toHaveBeenCalledWith(3, 1); expect(changed).toHaveBeenCalledWith([tracks[1]]);
  });
  it("upload API errors do not invent a track", async () => {
    const changed = vi.fn(); vi.mocked(api.uploadSubtitle).mockRejectedValue(new Error("字幕上传失败"));
    render(<SubtitleManager video={videoFixture()} onTracksChanged={changed} workspace />);
    fireEvent.change(screen.getByLabelText("字幕文件"), { target: { files: [new File(["text"], "test.srt")] } }); fireEvent.submit(document.querySelector(".subtitle-upload-form")!);
    expect(await screen.findByText("字幕上传失败")).toBeTruthy(); expect(changed).not.toHaveBeenCalled();
  });
});
