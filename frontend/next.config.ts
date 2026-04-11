import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8081/api/:path*",
      },
      {
        source: "/health",
        destination: "http://localhost:8081/health",
      },
    ];
  },
};

export default nextConfig;
