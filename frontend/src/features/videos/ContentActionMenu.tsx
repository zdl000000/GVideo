import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { Ellipsis, MessageCircle, Pencil, RefreshCw, Trash2 } from "lucide-react";
import type { Video } from "../../types";

interface ContentActionMenuProps {
  video: Video;
  busy: string;
  onEdit: (trigger: HTMLButtonElement) => void;
  onSubtitles: (trigger: HTMLButtonElement) => void;
  onDelete: (trigger: HTMLButtonElement) => void;
  onRetry: () => void;
}

/** This menu belongs to the content workflow; dialog ownership stays with MyVideosPage. */
export function ContentActionMenu({ video, busy, onEdit, onSubtitles, onDelete, onRetry }: ContentActionMenuProps) {
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState({ top: 0, left: 0 });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const initialFocus = useRef<"first" | "last">("first");
  const menuId = useId();
  const close = () => { setOpen(false); triggerRef.current?.focus(); };
  const enabledItems = () => Array.from(menuRef.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not(:disabled)') ?? []);

  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      if (!triggerRef.current || !menuRef.current) return;
      const anchor = triggerRef.current.getBoundingClientRect();
      const menu = menuRef.current.getBoundingClientRect();
      const below = anchor.bottom + 6;
      setPosition({
        left: Math.max(12, Math.min(anchor.right - menu.width, window.innerWidth - menu.width - 12)),
        top: Math.max(12, Math.min(below + menu.height <= window.innerHeight - 12 ? below : anchor.top - menu.height - 6, window.innerHeight - menu.height - 12))
      });
    };
    place();
    const items = enabledItems();
    (initialFocus.current === "last" ? items.at(-1) : items[0])?.focus();
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    return () => { window.removeEventListener("resize", place); window.removeEventListener("scroll", place, true); };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const outside = (event: PointerEvent) => { if (!rootRef.current?.contains(event.target as Node)) close(); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") { event.preventDefault(); close(); } };
    document.addEventListener("pointerdown", outside);
    window.addEventListener("keydown", escape);
    return () => { document.removeEventListener("pointerdown", outside); window.removeEventListener("keydown", escape); };
  }, [open]);

  const select = (action: (trigger: HTMLButtonElement) => void) => {
    setOpen(false);
    // A dialog will take focus. Never queue focus to an item which is about to unmount.
    if (triggerRef.current) action(triggerRef.current);
  };
  return <div className="gv-content-actions" ref={rootRef} aria-label={`${video.title}的管理操作`}>
    <button ref={triggerRef} type="button" className="gv-content-menu-trigger" aria-label={`${video.title}的操作菜单`} aria-haspopup="menu" aria-expanded={open} aria-controls={open ? menuId : undefined}
      onClick={() => { initialFocus.current = "first"; setOpen((value) => !value); }}
      onKeyDown={(event) => { if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); initialFocus.current = event.key === "ArrowUp" ? "last" : "first"; setOpen(true); } }}><Ellipsis size={20} /></button>
    {open && <div ref={menuRef} id={menuId} role="menu" aria-label="投稿操作" className="gv-content-menu" style={position}
      onBlur={(event) => { if (event.relatedTarget && !rootRef.current?.contains(event.relatedTarget as Node)) setOpen(false); }}
      onKeyDown={(event) => {
        const items = enabledItems();
        const index = items.indexOf(document.activeElement as HTMLButtonElement);
        if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
          event.preventDefault();
          const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1 : (index + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
          items[next]?.focus();
        }
        if (event.key === "Tab") { event.preventDefault(); close(); }
      }}>
      <button type="button" role="menuitem" onClick={() => select(onEdit)}><Pencil size={16} />编辑视频</button>
      <button type="button" role="menuitem" onClick={() => select(onSubtitles)}><MessageCircle size={16} />字幕管理</button>
      {video.processing_status === "failed" && <button type="button" role="menuitem" disabled={busy === `retry-${video.id}`} onClick={() => { close(); onRetry(); }}><RefreshCw size={16} />重新处理</button>}
      <button type="button" role="menuitem" className="gv-content-menu-danger" disabled={video.processing_status === "processing" || busy === `delete-${video.id}`} title={video.processing_status === "processing" ? "转码完成后才能删除" : undefined} onClick={() => select(onDelete)}><Trash2 size={16} />删除</button>
    </div>}
  </div>;
}
