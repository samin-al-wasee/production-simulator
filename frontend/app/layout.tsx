import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "ForgeLab",
  description: "Production Sandbox: build and run a production system as a game",
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
              <Link href="/sandbox">Sandbox</Link>
              <Link href="/pipelines">Pipelines</Link>
              <Link href="/learning">Learning path</Link>
            </nav>
          </div>
        </header>
        <main>{children}</main>
      </body>
    </html>
  );
}
