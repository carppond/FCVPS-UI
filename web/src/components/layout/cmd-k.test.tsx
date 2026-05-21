/**
 * T-29: command-palette behaviour tests.
 *
 * We mock the API hooks and the router so the dialog can mount in isolation
 * without spinning a TanStack Router context (that's covered by integration
 * tests). The focus here is on open/close + group rendering + search filter.
 */
import * as React from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@/lib/i18n";
import i18n from "@/lib/i18n";
import cmdkEn from "@/locales/en/cmdk.json";

// ── mocks ───────────────────────────────────────────────────────────────────

const navigateFn = vi.fn();
vi.mock("@tanstack/react-router", async () => {
  return {
    useNavigate: () => navigateFn,
  };
});

vi.mock("@/api/node", () => ({
  useNodesQuery: (params: { search?: string }) => ({
    data: params.search
      ? {
          items: [
            { id: "n1", name: "hk-node-01", protocol: "vless" },
            { id: "n2", name: "jp-edge-02", protocol: "trojan" },
          ],
          total: 2,
        }
      : { items: [], total: 0 },
    isFetching: false,
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@/api/subscription", () => ({
  useSubscriptionsQuery: (params: { keyword?: string }) => ({
    data: params.keyword
      ? {
          items: [
            {
              id: "s1",
              name: "Airport-A",
              node_count: 12,
              last_sync_status: "ok",
            },
          ],
          total: 1,
        }
      : { items: [], total: 0 },
    isFetching: false,
    isLoading: false,
    isError: false,
  }),
  useSyncSubscriptionMutation: () => ({
    mutateAsync: vi.fn().mockResolvedValue({}),
    isPending: false,
  }),
}));

vi.mock("@/api/agent", () => ({
  useAgentsQuery: (params: { keyword?: string }) => ({
    data: params.keyword
      ? { items: [{ id: "a1", name: "agent-1", status: "online", online: true }], total: 1 }
      : { items: [], total: 0 },
    isFetching: false,
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@/api/ota", () => ({
  useOtaCheck: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/api/settings", () => ({
  useRotateSilentMode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  downloadBackup: vi.fn().mockResolvedValue(new Blob(["x"])),
}));

vi.mock("@/components/ui/toast", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    message: vi.fn(),
  },
}));

vi.mock("@/stores/auth-store", () => ({
  useAuthStore: Object.assign(
    () => ({ user: { id: "u1", username: "admin", role: "admin" }, token: "t" }),
    {
      getState: () => ({ user: { role: "admin" }, token: "t" }),
    },
  ),
}));

vi.mock("@/stores/ui-store", () => ({
  useUIStore: () => ({ theme: "dark", setTheme: vi.fn() }),
}));

// Re-import after mocks so the test hits the mocked symbols.
import { CmdK } from "./cmd-k";
import { useCmdK, useCmdKStore } from "@/hooks/use-cmd-k";

// ── helpers ─────────────────────────────────────────────────────────────────

function makeClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}

/** Test harness — mounting CmdK alone is enough since opening relies on the
 * global Cmd+K shortcut handler registered by the component itself. */
function Harness() {
  const setOpen = useCmdK().setOpen;
  return (
    <div>
      <button data-testid="open" onClick={() => setOpen(true)}>
        open
      </button>
      <CmdK />
    </div>
  );
}

beforeAll(async () => {
  if (!i18n.hasResourceBundle("en", "cmdk")) {
    i18n.addResourceBundle("en", "cmdk", cmdkEn, true, true);
  }
  await i18n.changeLanguage("en");
});

beforeEach(() => {
  navigateFn.mockReset();
  // Reset cmdk store between tests so a previously-open dialog does not bleed
  // into the next case (would cause Cmd+K to *close* instead of open it).
  act(() => {
    useCmdKStore.setState({ open: false });
  });
  window.localStorage.clear();
});

function renderPalette() {
  const client = makeClient();
  render(
    <QueryClientProvider client={client}>
      <Harness />
    </QueryClientProvider>,
  );
}

// ── tests ───────────────────────────────────────────────────────────────────

describe("CmdK", () => {
  it("starts closed and opens via the Cmd+K shortcut", () => {
    renderPalette();
    expect(screen.queryByPlaceholderText(cmdkEn.placeholder)).toBeNull();
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    expect(screen.getByPlaceholderText(cmdkEn.placeholder)).toBeInTheDocument();
  });

  it("renders the static groups (pages + actions + admin) when open", () => {
    renderPalette();
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    // Headings rendered uppercase by Tailwind; assert via the localized text.
    expect(screen.getByText(cmdkEn.groups.pages, { selector: "[cmdk-group-heading]" })).toBeInTheDocument();
    expect(screen.getByText(cmdkEn.groups.actions, { selector: "[cmdk-group-heading]" })).toBeInTheDocument();
    expect(screen.getByText(cmdkEn.groups.admin, { selector: "[cmdk-group-heading]" })).toBeInTheDocument();
  });

  it("filters down to the resources group once the user types a query", async () => {
    const user = userEvent.setup();
    renderPalette();
    fireEvent.keyDown(document, { key: "k", metaKey: true });

    const input = screen.getByPlaceholderText(cmdkEn.placeholder);
    await user.type(input, "Airport");
    // Resources group should now appear (debounce is 200ms; advance timers
    // is overkill — userEvent.type already paces inputs slowly enough for
    // the debounced search to settle in jsdom).
    await new Promise((r) => setTimeout(r, 250));
    expect(
      screen.getByText(cmdkEn.groups.resources, { selector: "[cmdk-group-heading]" }),
    ).toBeInTheDocument();
    // The mocked subscription "Airport-A" should be among the results.
    expect(screen.getByText("Airport-A")).toBeInTheDocument();
  });

  it("closes via Escape", () => {
    renderPalette();
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    const input = screen.getByPlaceholderText(cmdkEn.placeholder);
    fireEvent.keyDown(input, { key: "Escape" });
    expect(screen.queryByPlaceholderText(cmdkEn.placeholder)).toBeNull();
  });
});
