import type { ButtonHTMLAttributes, Ref } from "react";

export function IconButton({ label, className = "", type = "button", ref, ...props }: Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> & { label: string; ref?: Ref<HTMLButtonElement> }) {
  return <button {...props} ref={ref} type={type} aria-label={label} title={props.title ?? label} className={`gv-icon-button icon-button ${className}`.trim()} />;
}
