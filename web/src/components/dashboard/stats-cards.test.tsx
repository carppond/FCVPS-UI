/**
 * Dashboard v4: stats-cards rendering tests.
 *
 * We mock the data hooks (subscriptions / agents / nodes / traffic / events)
 * and assert the tile contents render against the design. Loading states
 * are tested to confirm skeleton fallbacks work.
 */
import * as React from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@/lib/i18n";
import i18n from "@/lib/i18n";
import dashboardEn from "@/locales/en/dashboard.json";

// ── mocks ───────────────────────────────────────────────────────────────────

const subscriptionsMock = vi.fn();
const agentsMock = vi.fn();
const nodesMock = vi.fn();
const trafficMock = vi.fn();
const eventsMock = vi.fn();

vi.mock("@/api/subscription", () => ({
  useSubscriptionsQuery: () => subscriptionsMock(),
}));
vi.mock("@/api/agent", () => ({
  useAgentsQuery: () => agentsMock(),
}));
vi.mock("@/api/node", () => ({
  useNodesQuery: () => nodesMock(),
}));
vi.mock("@/api/traffic", () => ({
  useTrafficSummaryQuery: () => trafficMock(),
}));
vi.mock("@/api/notify", () => ({
  useEvents: () => eventsMock(),
}));

// Avoid the router throwing because <Link> is rendered in each card.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
}));

import { StatsCards } from "./stats-cards";

// ── helpers ─────────────────────────────────────────────────────────────────

function client() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

function renderWithProviders() {
  return render(
    <QueryClientProvider client={client()}>
      <StatsCards />
    </QueryClientProvider>,
  );
}

beforeAll(async () => {
  if (!i18n.hasResourceBundle("en", "dashboard")) {
    i18n.addResourceBundle("en", "dashboard", dashboardEn, true, true);
  }
  await i18n.changeLanguage("en");
});

beforeEach(() => {
  subscriptionsMock.mockReset();
  agentsMock.mockReset();
  nodesMock.mockReset();
  trafficMock.mockReset();
  eventsMock.mockReset();
});

// ── tests ───────────────────────────────────────────────────────────────────

describe("StatsCards", () => {
  it("renders all four card titles when data is available", () => {
    subscriptionsMock.mockReturnValue({
      data: {
        items: [
          { id: "s1", name: "sub1", last_sync_status: "ok", node_count: 5 },
          { id: "s2", name: "sub2", last_sync_status: "error", node_count: 3 },
        ],
        total: 2,
      },
      isLoading: false,
      isError: false,
    });
    agentsMock.mockReturnValue({
      data: {
        items: [
          { id: "a1", name: "a1", status: "online", online: true },
          { id: "a2", name: "a2", status: "offline", online: false },
        ],
      },
      isLoading: false,
      isError: false,
    });
    nodesMock.mockReturnValue({
      data: { items: [{ id: "n1", name: "x", protocol: "vless" }], total: 10 },
      isLoading: false,
      isError: false,
    });
    trafficMock.mockReturnValue({
      data: {
        total_used: 500 * 1024 * 1024,
        total_limit: 1024 * 1024 * 1024,
        usage_percent: 50,
      },
      isLoading: false,
      isError: false,
    });
    eventsMock.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    });

    renderWithProviders();

    expect(screen.getByText(dashboardEn.stats.subscriptions.title)).toBeInTheDocument();
    expect(screen.getByText(dashboardEn.stats.nodes.title)).toBeInTheDocument();
    expect(screen.getByText(dashboardEn.stats.traffic.title)).toBeInTheDocument();
    expect(screen.getByText(dashboardEn.stats.alerts.title)).toBeInTheDocument();
  });

  it("renders skeletons while tiles are loading", () => {
    subscriptionsMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    agentsMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    nodesMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    trafficMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    eventsMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });

    const { container } = renderWithProviders();
    expect(container.querySelectorAll(".animate-pulse").length).toBeGreaterThanOrEqual(4);
  });

  it("renders zero alerts as 'All clear'", () => {
    subscriptionsMock.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    });
    agentsMock.mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
    });
    nodesMock.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    });
    trafficMock.mockReturnValue({
      data: { total_used: 0, total_limit: 0, usage_percent: 0 },
      isLoading: false,
      isError: false,
    });
    eventsMock.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    });

    renderWithProviders();
    expect(screen.getByText(dashboardEn.stats.alerts.all_clear)).toBeInTheDocument();
  });
});
