import * as React from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@/lib/i18n";
import i18n from "@/lib/i18n";
import agentEn from "@/locales/en/agent.json";
import { AgentCreateDialog } from "./agent-create-dialog";

// Stub the agent API hook so the wizard mounts without a backend round-trip.
// We capture the create payload to assert that step-1 choices (name + kind)
// flow into the POST body and that the one-shot token is rendered exactly
// once on step-2 — never again after dismissal.
const createMutateAsync = vi.fn();

vi.mock("@/api/agent", async () => {
  const actual = await vi.importActual<typeof import("@/api/agent")>(
    "@/api/agent",
  );
  return {
    ...actual,
    useCreateAgentMutation: () => ({
      mutateAsync: createMutateAsync,
      isPending: false,
    }),
  };
});

// jsdom does not implement clipboard.writeText for older versions; stub it so
// the copy buttons can render their onClick without crashing.
beforeAll(async () => {
  if (!i18n.hasResourceBundle("en", "agent")) {
    i18n.addResourceBundle("en", "agent", agentEn, true, true);
  }
  await i18n.changeLanguage("en");
  if (
    typeof navigator !== "undefined" &&
    !Object.getOwnPropertyDescriptor(navigator, "clipboard")
  ) {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    });
  }
});

beforeEach(() => {
  createMutateAsync.mockReset();
});

function renderDialog(
  props?: Partial<React.ComponentProps<typeof AgentCreateDialog>>,
) {
  const onClose = vi.fn();
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <AgentCreateDialog open onClose={onClose} {...props} />
    </QueryClientProvider>,
  );
  return { onClose };
}

describe("AgentCreateDialog", () => {
  it("renders step 1 with native selected by default", () => {
    renderDialog();
    expect(screen.getByTestId("wizard-step-1")).toBeInTheDocument();
    expect(screen.getByTestId("wizard-kind-native")).toBeInTheDocument();
    expect(screen.getByTestId("wizard-kind-nezha")).toBeInTheDocument();
  });

  it("advances step 1 → step 2 → step 3 and exposes the one-shot token", async () => {
    const user = userEvent.setup();
    renderDialog();

    // Name is required; without it the Create button stays disabled.
    const submit = screen.getByTestId("wizard-next-1");
    expect(submit).toBeDisabled();

    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, "home-server");

    // Switch to nezha_compat so we can also assert the migration hint shows.
    await user.click(screen.getByTestId("wizard-kind-nezha"));

    createMutateAsync.mockResolvedValueOnce({
      id: "ag_123",
      user_id: "u1",
      name: "home-server",
      kind: "nezha_compat",
      status: "offline",
      created_at: 1716190000000,
      updated_at: 1716190000000,
      token: "plaintext-secret-xyz",
      install_command: "Server: https://<hub>/api/v1/nezha",
      install_hint_i18n_key: "agent.nezha_compat.install_hint",
    });

    await user.click(submit);

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledTimes(1),
    );
    const payload = createMutateAsync.mock.calls[0][0];
    expect(payload.name).toBe("home-server");
    expect(payload.kind).toBe("nezha_compat");

    // Step 2: token rendered exactly once + nezha migration hint visible.
    await waitFor(() =>
      expect(screen.getByTestId("wizard-step-2")).toBeInTheDocument(),
    );
    const tokenEl = screen.getByTestId("wizard-token");
    expect(tokenEl).toHaveTextContent("plaintext-secret-xyz");
    expect(screen.getByTestId("wizard-nezha-hint")).toBeInTheDocument();

    // Advancing to step 3 should drop the token from the DOM.
    await user.click(screen.getByTestId("wizard-next-2"));
    expect(screen.getByTestId("wizard-step-3")).toBeInTheDocument();
    expect(screen.queryByTestId("wizard-token")).not.toBeInTheDocument();
  });

  it("renders the install command (not nezha hint) when kind=native", async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.type(screen.getByLabelText(/name/i), "edge-1");

    createMutateAsync.mockResolvedValueOnce({
      id: "ag_456",
      user_id: "u1",
      name: "edge-1",
      kind: "native",
      status: "offline",
      created_at: 1716190000000,
      updated_at: 1716190000000,
      token: "another-secret",
      install_command:
        "curl -fsSL https://example/install-agent.sh | bash -s -- --token=another-secret",
    });

    await user.click(screen.getByTestId("wizard-next-1"));

    await waitFor(() =>
      expect(screen.getByTestId("wizard-step-2")).toBeInTheDocument(),
    );
    const cmd = screen.getByTestId("wizard-install-command");
    expect(cmd).toHaveTextContent("install-agent.sh");
    expect(screen.queryByTestId("wizard-nezha-hint")).not.toBeInTheDocument();
  });
});
