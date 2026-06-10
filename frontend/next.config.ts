import type { NextConfig } from "next";

// Where the Next server proxies /api/* and /health. Resolved at build time
// (standalone output serializes the config), so containerized builds pass
// DASHBOARD_URL=http://dashboard:8081 as a build arg; local dev defaults to
// the host-run dashboard.
const DASHBOARD_URL = process.env.DASHBOARD_URL ?? "http://localhost:8081";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${DASHBOARD_URL}/api/:path*`,
      },
      {
        source: "/health",
        destination: `${DASHBOARD_URL}/health`,
      },
    ];
  },
};

export default nextConfig;
