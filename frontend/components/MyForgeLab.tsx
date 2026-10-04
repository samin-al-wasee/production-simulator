"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { fetchMe, fetchProviders, type AuthProvider, type AuthUser } from "@/lib/auth";
import { rememberGame, sandboxApi, type SavedSummary } from "@/lib/sandbox";

function providerLabel(name: string): string {
  if (name === "github") return "GitHub";
  if (name === "google") return "Google";
  return name;
}

function SignIn({ providers }: { providers: AuthProvider[] }) {
  return (
    <section className="card">
      <h3>Sign in to keep your work</h3>
      <p>
        Playing is anonymous and free — your game lives in memory only. Sign in to save games, resume them later,
        and keep learning progress across reloads.
      </p>
      {providers.length === 0 ? (
        <p className="muted">Sign-in is not configured on this server.</p>
      ) : (
        <p className="signin-links">
          {providers.map((p) => (
            <a key={p.name} className="signin" href={p.url}>
              Sign in with {providerLabel(p.name)}
            </a>
          ))}
        </p>
      )}
    </section>
  );
}

export function MyForgeLab() {
  const router = useRouter();
  const [ready, setReady] = useState(false);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [saves, setSaves] = useState<SavedSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busyID, setBusyID] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [available, me] = await Promise.all([fetchProviders(), fetchMe()]);
      if (cancelled) return;
      setProviders(available);
      setUser(me);
      if (me) {
        try {
          const list = await sandboxApi.saves();
          if (!cancelled) setSaves(list);
        } catch (err) {
          if (!cancelled) setError(err instanceof Error ? err.message : String(err));
        }
      }
      setReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function resume(id: string) {
    setBusyID(id);
    setError(null);
    try {
      const g = await sandboxApi.resume(id);
      rememberGame(g.id);
      router.push("/sandbox");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusyID(null);
    }
  }

  async function remove(id: string) {
    setBusyID(id);
    setError(null);
    try {
      await sandboxApi.deleteSave(id);
      setSaves((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusyID(null);
    }
  }

  if (!ready) return <p className="muted">Loading…</p>;
  if (!user) return <SignIn providers={providers} />;

  return (
    <>
      <p className="lead">
        Signed in as {user.displayName || user.email}. Your saved games follow your account.
      </p>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {saves.length === 0 ? (
        <section className="card">
          <h3>No saved games yet</h3>
          <p>
            Open the <Link href="/sandbox">Sandbox</Link> and press Save to keep a game here.
          </p>
        </section>
      ) : (
        <section className="card">
          <h3>Saved games</h3>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Ruleset</th>
                <th>Tick</th>
                <th>Updated</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {saves.map((s) => (
                <tr key={s.id}>
                  <td>{s.name || "Untitled"}</td>
                  <td>{s.ruleset}</td>
                  <td>{s.tick}</td>
                  <td>{new Date(s.updatedAt).toLocaleString()}</td>
                  <td className="actions">
                    <button className="secondary" disabled={busyID === s.id} onClick={() => resume(s.id)}>
                      Resume
                    </button>
                    <button className="secondary" disabled={busyID === s.id} onClick={() => remove(s.id)}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </>
  );
}
