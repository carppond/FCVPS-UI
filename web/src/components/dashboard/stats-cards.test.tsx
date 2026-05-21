/**
 * T-29: stats-cards rendering tests.
 *
 * We mock the four data hooks (agents / nodes / traffic / events) and assert
 * the tile contents render against the design (numbers, status copy, and
 * progress bar). Loading + error fallbacks each have a dedicated case so a
 * regression in any state shows up immediately.
 */
import * as React from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@/lib/i18n";
import i18n from "@/lib/i18n";
import dashboardEn from "@/locales/en/dashboard.json";

// ── mocks ───────────────────────────────────────────────────────────────────

const agentsMock = vi.fn();
const nodesMock = vi.fn();
const trafficMock = vi.fn();
const eventsMock = vi.fn();

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
  agentsMock.mockReset();
  nodesMock.mockReset();
  trafficMock.mockReset();
  eventsMock.mockReset();
});

// ── tests ───────────────────────────────────────────────────────────────────

describe("StatsCards", () => {
  it("renders all four titles when data is available", () => {
    agentsMock.mockReturnValue({
      data: {
        items: [
          { id: "1", name: "a1", status: "online", online: true },
          { id: "2", name: "a2", status: "online", online: true },
          { id: "3", name: "a3", status: "offline", online: false },
        ],
      },
      isLoading: false,
      isError: false,
    });
    nodesMock.mockReturnValue({
      data: {
        items: [
          { id: "n1", name: "x", protocol: "vless" },
          { id: "n2", name: "y", protocol: "trojan" },
        ],
        total: 14,
      },
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
      data: { items: [], total: 2 },
      isLoading: false,
      isError: false,
    });

    renderWithProviders();

    expect(screen.getByText(dashboardEn.stats.agents.title)).toBeInTheDocument();
    expect(screen.getByText(dashboardEn.stats.nodes.title)).toBeInTheDocument();
    expect(screen.getByText(dashboardEn.stats.traffic.title)).toBeInTheDocument();
    expect(screen.getByText(dashboardEn.stats.alerts.title)).toBeInTheDocument();

    // Two of three online → "2 / 3".
    expect(screen.getByText("2 / 3")).toBeInTheDocument();
    // Total nodes number.
    expect(screen.getByText("14")).toBeInTheDocument();
    // Alerts count
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders skeletons while a tile is loading", () => {
    agentsMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    nodesMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    trafficMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    eventsMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });

    const { container } = renderWithProviders();
    // The Skeleton helper uses animate-pulse; we count its appearances to
    // confirm every tile entered loading mode.
    expect(container.querySelectorAll(".animate-pulse").length).toBeGreaterThanOrEqual(4);
  });

  it("renders the error pill when a tile query fails", () => {
    agentsMock.mockReturnValue({ data: undefined, isLoading: false, isError: true });
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

    // The agents tile should render the error copy.
    const errors = screen.getAllByText(dashboardEn.error.load_failed);
    expect(errors.length).toBeGreaterThan(0);
  });

  it("renders the no-limit hint when traffic has no quota configured", () => {
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
      data: { total_used: 100, usage_percent: 0 },
      isLoading: false,
      isError: false,
    });
    eventsMock.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    });

    renderWithProviders();
    expect(screen.getByText(dashboardEn.stats.traffic.no_limit)).toBeInTheDocument();
  });
});
