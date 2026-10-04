"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { fetchMe, fetchProviders, logout, type AuthProvider, type AuthUser } from "@/lib/auth";

// UserMenu is the header sign-in/out control (ADR-0033). It renders nothing
// when sign-in is not configured (no database), so store-less runs look as
// before. Sign-in is an <a> because the endpoint is a top-level redirect.

function providerLabel(name: string): string {
  if (name === "github") return "GitHub";
  if (name === "google") return "Google";
  return name;
}

export function UserMenu() {
  const router = useRouter();
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [available, me] = await Promise.all([fetchProviders(), fetchMe().catch(() => null)]);
      if (cancelled) return;
      setProviders(available);
      setUser(me);
      setReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (!ready || (!user && providers.length === 0)) return null;

  if (!user) {
    return (
      <div className="user-menu">
        {providers.map((p) => (
          <a key={p.name} className="signin" href={p.url}>
            Sign in with {providerLabel(p.name)}
          </a>
        ))}
      </div>
    );
  }

  return (
    <div className="user-menu">
      <span className="who">{user.displayName || user.email || "Signed in"}</span>
      <button
        className="secondary"
        onClick={async () => {
          await logout();
          setUser(null);
          router.refresh();
        }}
      >
        Sign out
      </button>
    </div>
  );
}
