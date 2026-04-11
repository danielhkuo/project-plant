"use client";

import { SWRConfig } from "swr";
import { MockProvider } from "./MockProvider";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map() }}>
      <MockProvider>{children}</MockProvider>
    </SWRConfig>
  );
}
