import { Suspense } from "react";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { SWRConfig } from "swr";
import { afterEach, describe, it, expect, vi } from "vitest";
import DeviceDetailPage from "@/app/devices/[id]/page";
import { useDeviceHistory } from "@/hooks/useDeviceHistory";
import { server } from "./mocks/server";
import { mockHistory } from "./mocks/fixtures";

const HOUR = 60 * 60 * 1000;
// The dashboard API orders history by recorded_at DESC.
const newestFirst = [...mockHistory].reverse();

// Fresh SWR cache per test so one test's responses never satisfy another's.
function IsolatedCache({ children }: { children: React.ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map() }}>{children}</SWRConfig>
  );
}

// Rendering inside an async act() lets React finish the Suspense retry that
// use(params) triggers in the page.
async function renderPage(id: string) {
  await act(async () => {
    render(
      <IsolatedCache>
        <Suspense fallback={null}>
          <DeviceDetailPage params={Promise.resolve({ id })} />
        </Suspense>
      </IsolatedCache>
    );
  });
  await screen.findByRole("heading", { name: id });
}

afterEach(() => {
  vi.useRealTimers();
});

describe("DeviceDetailPage history", () => {
  it("fetches history once instead of refetching on every render", async () => {
    let historyRequests = 0;
    server.use(
      http.get("/api/v1/devices/:id/history", () => {
        historyRequests++;
        return HttpResponse.json(newestFirst);
      })
    );

    await renderPage("dev-001");
    await waitFor(() => expect(historyRequests).toBeGreaterThan(0));

    // Before the fix, every response re-rendered the page, which built a new
    // SWR key from Date.now() and started another fetch: dozens within 500 ms.
    await new Promise((r) => setTimeout(r, 500));
    expect(historyRequests).toBe(1);
  });

  it("requests a time window that matches the selected range", async () => {
    const spans: number[] = [];
    server.use(
      http.get("/api/v1/devices/:id/history", ({ request }) => {
        const q = new URL(request.url).searchParams;
        spans.push(Date.parse(q.get("to")!) - Date.parse(q.get("from")!));
        return HttpResponse.json(newestFirst);
      })
    );

    await renderPage("dev-001");
    await waitFor(() => expect(spans).toHaveLength(1));

    fireEvent.click(screen.getByRole("button", { name: "6H" }));
    await waitFor(() => expect(spans).toHaveLength(2));
    expect(spans).toEqual([1 * HOUR, 6 * HOUR]);
  });
});

describe("useDeviceHistory", () => {
  it("returns readings oldest-first when the API sends newest-first", async () => {
    server.use(
      http.get("/api/v1/devices/:id/history", () =>
        HttpResponse.json(newestFirst)
      )
    );

    const { result } = renderHook(() => useDeviceHistory("dev-001", 1), {
      wrapper: IsolatedCache,
    });

    await waitFor(() =>
      expect(result.current.history).toHaveLength(mockHistory.length)
    );
    expect(result.current.history).toEqual(mockHistory);
  });

  it("keeps readings oldest-first when they already arrive oldest-first", async () => {
    // Demo mode's mock API (src/mocks) serves history oldest-first.
    server.use(
      http.get("/api/v1/devices/:id/history", () =>
        HttpResponse.json(mockHistory)
      )
    );

    const { result } = renderHook(() => useDeviceHistory("dev-001", 1), {
      wrapper: IsolatedCache,
    });

    await waitFor(() =>
      expect(result.current.history).toHaveLength(mockHistory.length)
    );
    expect(result.current.history).toEqual(mockHistory);
  });

  it("refetches every 30 seconds so the window keeps moving", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let historyRequests = 0;
    server.use(
      http.get("/api/v1/devices/:id/history", () => {
        historyRequests++;
        return HttpResponse.json(newestFirst);
      })
    );

    renderHook(() => useDeviceHistory("dev-001", 1), {
      wrapper: IsolatedCache,
    });
    await waitFor(() => expect(historyRequests).toBe(1));

    await act(() => vi.advanceTimersByTimeAsync(30_000));
    await waitFor(() => expect(historyRequests).toBe(2));
  });
});
