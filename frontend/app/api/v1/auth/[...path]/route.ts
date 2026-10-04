import type { NextRequest } from "next/server";
import { API_URL } from "@/lib/api";

// Same-origin proxy for the core API's OAuth surface (ADR-0033). The session
// cookie is HttpOnly and SameSite=Lax, so it only reaches the dashboard if the
// whole flow stays on the dashboard origin: the backend builds its callback as
// `<dashboard>/api/v1/auth/callback/{provider}`, this route forwards it to the
// API, and it copies Set-Cookie and Location back verbatim. A plain rewrite
// would not promise Set-Cookie fidelity across a 302.
export const dynamic = "force-dynamic";

type RouteContext = { params: Promise<{ path: string[] }> };

async function proxy(req: NextRequest, ctx: RouteContext): Promise<Response> {
  const { path } = await ctx.params;
  const target = `${API_URL}/api/v1/auth/${path.map(encodeURIComponent).join("/")}${req.nextUrl.search}`;

  const headers = new Headers();
  const cookie = req.headers.get("cookie");
  if (cookie) headers.set("cookie", cookie);
  const contentType = req.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);

  const upstream = await fetch(target, {
    method: req.method,
    headers,
    body: req.method === "GET" || req.method === "HEAD" ? undefined : await req.arrayBuffer(),
    redirect: "manual",
  });

  const res = new Response(upstream.body, { status: upstream.status });
  for (const c of upstream.headers.getSetCookie()) res.headers.append("set-cookie", c);
  for (const name of ["location", "content-type"]) {
    const v = upstream.headers.get(name);
    if (v) res.headers.set(name, v);
  }
  return res;
}

export function GET(req: NextRequest, ctx: RouteContext) {
  return proxy(req, ctx);
}

export function POST(req: NextRequest, ctx: RouteContext) {
  return proxy(req, ctx);
}
