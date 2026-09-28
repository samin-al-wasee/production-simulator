import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";
import { ScaleBadge } from "@/components/ScaleBadge";

export const metadata: Metadata = {
  title: "ForgeLab",
  description: "Production Systems Laboratory dashboard",
};

export const dynamic = "force-dynamic";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="site">
          <div className="inner">
            <span className="brand">ForgeLab</span>
            <nav>
              <Link href="/">Overview</Link>
              <Link href="/experiments">Experiments</Link>
              <Link href="/pipelines">Pipelines</Link>
              <Link href="/learning">Learning path</Link>
            </nav>
            <ScaleBadge />
          </div>
        </header>
        <main>{children}</main>
      </body>
    </html>
  );
}
