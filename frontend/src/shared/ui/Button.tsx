import type { ButtonHTMLAttributes, ReactNode } from "react";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export function buttonClassName(variant: ButtonVariant = "primary", className = "") {
  return `gv-button gv-button--${variant} ${className}`.trim();
}

export function Button({ variant = "primary", children, className, type = "button", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; children?: ReactNode }) {
  return <button {...props} type={type} className={buttonClassName(variant, className)}>{children}</button>;
}
