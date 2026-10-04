import type { NextConfig } from "next";

// The dashboard never reimplements simulation logic: browser calls go to
// /api/forgelab/* and are proxied to the forgelab core API.
const apiUrl = process.env.FORGELAB_API_URL ?? "http://127.0.0.1:8090";

const nextConfig: NextConfig = {
  // A separate distDir lets the database-backed auth browser tests run their
  // own dev server beside the default one without sharing Next's dev lock.
  distDir: process.env.NEXT_DIST_DIR ?? ".next",
  async rewrites() {
    return [{ source: "/api/forgelab/:path*", destination: `${apiUrl}/api/v1/:path*` }];
  },
};

export default nextConfig;
