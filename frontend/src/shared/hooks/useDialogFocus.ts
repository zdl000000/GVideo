import { useEffect, useRef } from "react";

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
    const focusableSelector = "input,textarea,button,select,a[href],[tabindex]:not([tabindex='-1'])";
    panel.querySelector<HTMLElement>(focusableSelector)?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busyRef.current) {
        event.preventDefault();
        closeRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = Array.from(panel.querySelectorAll<HTMLElement>(focusableSelector)).filter((element) => !element.hasAttribute("disabled"));
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      previous.forEach(({ element, inert, hidden }) => {
        element.inert = inert;
        if (hidden === null) element.removeAttribute("aria-hidden"); else element.setAttribute("aria-hidden", hidden);
      });
    };
  }, [panelRef]);
}
