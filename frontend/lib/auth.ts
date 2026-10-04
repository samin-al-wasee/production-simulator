// Browser-side auth client. Sign-in is a top-level navigation (the endpoint
// 302s to the provider), so callers use signInUrl / <a href>, never fetch.
// Calls go through the same-origin /api/v1/auth proxy route (ADR-0033).

export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  avatarUrl: string;
}

export interface AuthProvider {
  name: string;
  url: string;
}

const BASE = "/api/v1/auth";

export async function fetchProviders(): Promise<AuthProvider[]> {
  const res = await fetch(`${BASE}/providers`);
  if (!res.ok) return [];
  return (await res.json()) as AuthProvider[];
}

// fetchMe returns the signed-in user, or null when anonymous (401).
export async function fetchMe(): Promise<AuthUser | null> {
  const res = await fetch(`${BASE}/me`);
  if (res.status === 401) return null;
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return (await res.json()) as AuthUser;
}

export async function logout(): Promise<void> {
  await fetch(`${BASE}/logout`, { method: "POST" });
}
