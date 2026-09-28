import type { NextConfig } from "next";

// The dashboard never reimplements simulation logic: browser calls go to
// /api/forgelab/* and are proxied to the forgelab core API.
const apiUrl = process.env.FORGELAB_API_URL ?? "http://127.0.0.1:8090";

const nextConfig: NextConfig = {
  async rewrites() {
    return [{ source: "/api/forgelab/:path*", destination: `${apiUrl}/api/v1/:path*` }];
  },
};

export default nextConfig;
