import * as React from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@/lib/i18n";
import i18n from "@/lib/i18n";
import subEn from "@/locales/en/subscription.json";
import { SubCreateWizard } from "./sub-create-wizard";

// Stub the subscription API hooks so the wizard can mount without a backend.
// We capture the mutation payload to assert step 1 → step 4 carries data
// through correctly.
const createMutateAsync = vi.fn();
const uploadMutateAsync = vi.fn();
const createRuleMutateAsync = vi.fn();

vi.mock("@/api/subscription", async () => {
  const actual = await vi.importActual<typeof import("@/api/subscription")>(
    "@/api/subscription",
  );
  return {
    ...actual,
    useCreateSubscriptionMutation: () => ({
      mutateAsync: createMutateAsync,
      isPending: false,
    }),
    useUploadSubscriptionMutation: () => ({
      mutateAsync: uploadMutateAsync,
      isPending: false,
    }),
    useSubscriptionTagSuggestionsQuery: () => ({
      data: ["hk", "jp"],
      isLoading: false,
    }),
  };
});

// Stub the rule API hooks so step 4 has data without a backend.
vi.mock("@/api/rule", async () => {
  const actual = await vi.importActual<typeof import("@/api/rule")>(
    "@/api/rule",
  );
  return {
    ...actual,
    useRuleTemplatesQuery: () => ({
      data: [
        {
          id: "cn-direct-foreign-proxy",
          name: "CN direct + foreign proxy",
          description: "Mainland direct, others via proxy",
          content: "DOMAIN-SUFFIX,cn,DIRECT\nGEOIP,CN,DIRECT\nMATCH,Proxy\n",
        },
        {
          id: "global-proxy",
          name: "Global proxy",
          description: "Everything via proxy",
          content: "MATCH,Proxy\n",
        },
      ],
      isLoading: false,
    }),
    useRulesQuery: () => ({ data: { items: [], total: 0 }, isLoading: false }),
    useCreateRuleMutation: () => ({
      mutateAsync: createRuleMutateAsync,
      isPending: false,
    }),
  };
});

beforeAll(async () => {
  // Register the subscription namespace so t('subscription:...') resolves.
  if (!i18n.hasResourceBundle("en", "subscription")) {
    i18n.addResourceBundle("en", "subscription", subEn, true, true);
  }
  await i18n.changeLanguage("en");
});

beforeEach(() => {
  createMutateAsync.mockReset();
  uploadMutateAsync.mockReset();
  createRuleMutateAsync.mockReset();
  createRuleMutateAsync.mockResolvedValue({ id: "rule_1" });
});

function renderWizard(props?: Partial<React.ComponentProps<typeof SubCreateWizard>>) {
  const onClose = vi.fn();
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <SubCreateWizard open onClose={onClose} {...props} />
    </QueryClientProvider>,
  );
  return { onClose };
}

describe("SubCreateWizard", () => {
  it("renders step 1 with three source choices on mount", () => {
    renderWizard();
    expect(screen.getByTestId("wizard-step-1")).toBeInTheDocument();
    expect(screen.getByTestId("wizard-source-url")).toBeInTheDocument();
    expect(screen.getByTestId("wizard-source-upload")).toBeInTheDocument();
    expect(screen.getByTestId("wizard-source-manual")).toBeInTheDocument();
  });

  it("advances from step 1 → step 2 → step 3 → step 4 and submits a url subscription with carried data", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Step 1: default selection is "url"; click Next.
    await user.click(screen.getByRole("button", { name: /next/i }));

    // Step 2: required fields visible (name + URL).
    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, "Airport Z");
    const urlInput = screen.getByLabelText(/subscription url/i);
    await user.type(urlInput, "https://example.com/sub");

    await user.click(screen.getByRole("button", { name: /next/i }));

    // Step 3: tag chip + interval select rendered.
    expect(
      screen.getByLabelText(new RegExp(subEn.wizard.tags.label, "i")),
    ).toBeInTheDocument();

    // Advance to step 4 (rule template).
    await user.click(screen.getByRole("button", { name: /next/i }));

    // Step 4 indicator should be present.
    expect(screen.getByTestId("wizard-step-4")).toBeInTheDocument();

    // Resolve the create mutation immediately.
    createMutateAsync.mockResolvedValueOnce({ id: "sub_123" });

    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalledTimes(1));
    const payload = createMutateAsync.mock.calls[0][0];
    expect(payload.name).toBe("Airport Z");
    expect(payload.type).toBe("url");
    expect(payload.source_url).toBe("https://example.com/sub");
    // Default interval (6h) survives to the create payload.
    expect(payload.sync_interval).toBe(21600);
  });

  it("blocks advancing from step 2 when required fields are missing", async () => {
    const user = userEvent.setup();
    renderWizard();
    // Step 1 → 2 (url is default).
    await user.click(screen.getByRole("button", { name: /next/i }));
    // Name and URL are both empty. Pressing Next stays on step 2.
    await user.click(screen.getByRole("button", { name: /next/i }));
    // We're still on step 2 (no step 3 controls).
    expect(
      screen.queryByLabelText(new RegExp(subEn.wizard.tags.label, "i")),
    ).not.toBeInTheDocument();
    expect(createMutateAsync).not.toHaveBeenCalled();
  });
});
