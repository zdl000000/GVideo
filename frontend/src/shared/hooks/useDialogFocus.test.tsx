// @vitest-environment jsdom
import { useRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useDialogFocus } from "./useDialogFocus";

afterEach(cleanup);

function Workflow({ busy = false, onClose = () => {} }: { busy?: boolean; onClose?: () => void }) {
  const panelRef = useRef<HTMLElement>(null);
  useDialogFocus(panelRef, busy, onClose);
  return <><button>背景操作</button><div className="dialog-backdrop"><section ref={panelRef} role="dialog" aria-label="测试工作流" aria-modal="true">
    <button disabled={busy}>取消</button>
    <input type="file" className="sr-only" tabIndex={-1} aria-label="隐藏文件输入" />
    <input aria-label="隐藏字段" style={{ display: "none" }} />
    <button disabled={busy}>保存</button>
  </section></div></>;
}

describe("useDialogFocus", () => {
  it("busy禁用所有控件时焦点停留dialog，Tab与Shift+Tab不会丢到body", () => {
    const close = vi.fn(); const view = render(<Workflow onClose={close} />);
    const first = screen.getByRole("button", { name: "取消" });
    const last = screen.getByRole("button", { name: "保存" });
    last.focus(); fireEvent.keyDown(window, { key: "Tab" }); expect(document.activeElement).toBe(first);
    fireEvent.keyDown(window, { key: "Tab", shiftKey: true }); expect(document.activeElement).toBe(last);
    view.rerender(<Workflow busy onClose={close} />);
    const panel = screen.getByRole("dialog"); expect(document.activeElement).toBe(panel);
    for (const shiftKey of [false, true]) {
      const event = new KeyboardEvent("keydown", { key: "Tab", shiftKey, cancelable: true });
      window.dispatchEvent(event); expect(event.defaultPrevented).toBe(true); expect(document.activeElement).toBe(panel);
    }
    fireEvent.keyDown(window, { key: "Escape" }); expect(close).not.toHaveBeenCalled();
    view.rerender(<Workflow onClose={close} />);
    fireEvent.keyDown(window, { key: "Tab" }); expect(document.activeElement).toBe(first);
    fireEvent.keyDown(window, { key: "Escape" }); expect(close).toHaveBeenCalledOnce();
  });

  it("隐藏文件和display:none字段不进入可用焦点环", () => {
    render(<Workflow />);
    const first = screen.getByRole("button", { name: "取消" });
    const last = screen.getByRole("button", { name: "保存" });
    const hiddenFile = screen.getByLabelText("隐藏文件输入"); hiddenFile.focus();
    fireEvent.keyDown(window, { key: "Tab" }); expect(document.activeElement).toBe(first);
    hiddenFile.focus(); fireEvent.keyDown(window, { key: "Tab", shiftKey: true }); expect(document.activeElement).toBe(last);
  });

  it("卸载恢复背景先前的inert和aria-hidden，清除临时panel tabindex", () => {
    const view = render(<Workflow />); const background = screen.getByText("背景操作");
    const panel = screen.getByRole("dialog"); expect(background.inert).toBe(true); expect(background.getAttribute("aria-hidden")).toBe("true");
    expect(panel.tabIndex).toBe(-1); view.unmount();
    expect(background.inert).not.toBe(true); expect(background.getAttribute("aria-hidden")).toBeNull(); expect(panel.hasAttribute("tabindex")).toBe(false);
  });
});
