import Link from "next/link";
import type { ButtonHTMLAttributes, ReactNode } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode; variant?: "primary" | "secondary" | "dark"; href?: string };

export function Button({ children, variant = "primary", href, className = "", ...props }: Props) {
  const styles = `button button-${variant} ${className}`;
  if (href) return <Link href={href} className={styles}>{children}</Link>;
  return <button className={styles} {...props}>{children}</button>;
}

