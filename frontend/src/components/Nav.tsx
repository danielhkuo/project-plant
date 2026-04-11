"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/", label: "DASHBOARD" },
  { href: "/alerts", label: "ALERTS" },
];

export function Nav() {
  const pathname = usePathname();

  return (
    <nav className="flex items-center justify-between border-b border-border px-4 py-4 md:px-6">
      <Link href="/" className="font-display text-2xl tracking-tight text-text-display">
        PROJECT PLANT
      </Link>
      <div className="flex items-center gap-6">
        {links.map(({ href, label }) => {
          const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
          return (
            <Link
              key={href}
              href={href}
              className={`font-mono text-[13px] uppercase tracking-[0.06em] transition-colors ${
                active ? "text-text-display" : "text-text-disabled hover:text-text-secondary"
              }`}
            >
              {label}
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
