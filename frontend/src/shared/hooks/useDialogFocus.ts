import { useEffect, useRef } from "react";

function dialogFocusables(panel: HTMLElement) {
  return Array.from(panel.querySelectorAll<HTMLElement>("input,textarea,button,select,a[href],[tabindex]"))
    .filter((element) => element.tabIndex >= 0 && !element.matches(":disabled,[hidden],[aria-hidden='true']")
      && getComputedStyle(element).display !== "none" && getComputedStyle(element).visibility !== "hidden");
}

export function useDialogFocus(panelRef: React.RefObject<HTMLElement | null>, busy: boolean, onClose: () => void) {
  const busyRef = useRef(busy);
  const closeRef = useRef(onClose);
  busyRef.current = busy;
  closeRef.current = onClose;

  useEffect(() => {
    const panel = panelRef.current;
    if (!panel) return;
    const backdrop = panel.closest<HTMLElement>(".dialog-backdrop");
    const background: HTMLElement[] = [];
    let foreground: HTMLElement | null = backdrop;
    while (foreground?.parentElement) {
      background.push(...Array.from(foreground.parentElement.children).filter((child) => child !== foreground) as HTMLElement[]);
      foreground = foreground.parentElement;
      if (foreground === document.body) break;
    }
    const previous = background.map((element) => ({ element, inert: element.inert, hidden: element.getAttribute("aria-hidden") }));
    background.forEach((element) => { element.inert = true; element.setAttribute("aria-hidden", "true"); });
    const previousTabIndex = panel.getAttribute("tabindex");
    // The panel remains a focus target when a busy workflow disables every control.
    panel.tabIndex = -1;
    (dialogFocusables(panel)[0] ?? panel).focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busyRef.current) {
        event.preventDefault();
        closeRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = dialogFocusables(panel);
      if (!focusable.length) { event.preventDefault(); panel.focus(); return; }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!focusable.includes(document.activeElement as HTMLElement)) { event.preventDefault(); (event.shiftKey ? last : first).focus(); }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      if (previousTabIndex === null) panel.removeAttribute("tabindex"); else panel.setAttribute("tabindex", previousTabIndex);
      previous.forEach(({ element, inert, hidden }) => {
        element.inert = inert;
        if (hidden === null) element.removeAttribute("aria-hidden"); else element.setAttribute("aria-hidden", hidden);
      });
    };
  }, [panelRef]);

  useEffect(() => {
    const panel = panelRef.current;
    if (!panel) return;
    const focusables = dialogFocusables(panel);
    if (!panel.contains(document.activeElement) || (document.activeElement !== panel && !focusables.includes(document.activeElement as HTMLElement))) {
      (focusables[0] ?? panel).focus();
    }
  }, [busy, panelRef]);
}
