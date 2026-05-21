import * as React from "react";
import { describe, it, expect, beforeAll, beforeEach, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import i18n from "@/lib/i18n";
import notifyEn from "@/locales/en/notify.json";

import { ChannelForm } from "./channel-form";
import type { ChannelKindDescriptor } from "@/api/notify";

// ---- mock the notify api module so the form sees a deterministic schema ----

const TELEGRAM_DESCRIPTOR: ChannelKindDescriptor = {
  kind: "telegram",
  display_name: "Telegram",
  fields: [
    { name: "bot_token", type: "password", required: true },
    { name: "chat_id", type: "string", required: true },
    { name: "parse_mode", type: "select", required: false, options: [
      { value: "HTML", label: "HTML" },
      { value: "Markdown", label: "Markdown" },
    ] },
  ],
};

const EMAIL_DESCRIPTOR: ChannelKindDescriptor = {
  kind: "email",
  display_name: "Email",
  fields: [
    { name: "smtp_host", type: "string", required: true },
    { name: "smtp_port", type: "number", required: true, default: 465 },
    { name: "smtp_password", type: "password", required: true },
    { name: "smtp_tls", type: "boolean", required: false, default: true },
    { name: "to", type: "string[]", required: true },
  ],
};

const WEBHOOK_DESCRIPTOR: ChannelKindDescriptor = {
  kind: "webhook",
  display_name: "Webhook",
  fields: [
    { name: "url", type: "string", required: true },
    { name: "method", type: "select", required: false, options: [
      { value: "POST" }, { value: "GET" },
    ] },
    { name: "headers", type: "map", required: false },
  ],
};

vi.mock("@/api/notify", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/notify")>();
  return {
    ...actual,
    useChannelKinds: () => ({
      data: [TELEGRAM_DESCRIPTOR, EMAIL_DESCRIPTOR, WEBHOOK_DESCRIPTOR],
      isLoading: false,
      isError: false,
    }),
    useCreateChannel: () => ({
      mutateAsync: vi.fn(),
      isPending: false,
    }),
    useUpdateChannel: () => ({
      mutateAsync: vi.fn(),
      isPending: false,
    }),
  };
});

beforeAll(async () => {
  if (!i18n.hasResourceBundle("en", "notify")) {
    i18n.addResourceBundle("en", "notify", notifyEn, true, true);
  }
  await i18n.changeLanguage("en");
});

function renderWithClient(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ChannelForm", () => {
  it("renders the Telegram fields by default", () => {
    renderWithClient(<ChannelForm channel={null} initialKind="telegram" />);

    expect(screen.getByTestId("notify-field-bot_token")).toBeInTheDocument();
    expect(screen.getByTestId("notify-field-chat_id")).toBeInTheDocument();
    expect(screen.getByTestId("notify-field-parse_mode")).toBeInTheDocument();

    // password input is rendered with the password type so secret leakage
    // via screenshot / DOM inspection is mitigated.
    const botToken = screen.getByTestId(
      "notify-field-bot_token",
    ) as HTMLInputElement;
    expect(botToken.type).toBe("password");
  });

  it("swaps fields when the kind is changed (email)", () => {
    renderWithClient(<ChannelForm channel={null} initialKind="telegram" />);

    const kindSelect = screen.getByTestId(
      "notify-form-kind",
    ) as HTMLSelectElement;
    fireEvent.change(kindSelect, { target: { value: "email" } });

    expect(screen.queryByTestId("notify-field-bot_token")).toBeNull();
    expect(screen.getByTestId("notify-field-smtp_host")).toBeInTheDocument();
    expect(screen.getByTestId("notify-field-smtp_port")).toBeInTheDocument();
    expect(screen.getByTestId("notify-field-smtp_password")).toBeInTheDocument();
    expect(screen.getByTestId("notify-field-smtp_tls")).toBeInTheDocument();
    expect(screen.getByTestId("notify-field-to")).toBeInTheDocument();

    // Number input declared by schema should render as type=number.
    const port = screen.getByTestId(
      "notify-field-smtp_port",
    ) as HTMLInputElement;
    expect(port.type).toBe("number");
  });

  it("renders the webhook map field as a textarea", () => {
    renderWithClient(<ChannelForm channel={null} initialKind="telegram" />);

    const kindSelect = screen.getByTestId(
      "notify-form-kind",
    ) as HTMLSelectElement;
    fireEvent.change(kindSelect, { target: { value: "webhook" } });

    const headers = screen.getByTestId("notify-field-headers");
    expect(headers.tagName).toBe("TEXTAREA");
  });

  it("blocks submission and shows required-field errors when blank", () => {
    renderWithClient(<ChannelForm channel={null} initialKind="telegram" />);

    const submit = screen.getByRole("button", { name: /create/i });
    fireEvent.click(submit);

    // Form should not have called create yet — error messages should be
    // visible inline. The submit handler returns before the mutation fires.
    expect(
      screen.getByText(/channel name is required/i),
    ).toBeInTheDocument();
  });

  it("locks the kind picker when editing an existing channel", () => {
    renderWithClient(
      <ChannelForm
        channel={{
          id: "ch_1",
          user_id: "u",
          kind: "email",
          name: "ops",
          config: { smtp_host: "smtp.example.com" } as never,
          event_types: [],
          enabled: true,
          created_at: 0,
          updated_at: 0,
        }}
      />,
    );

    const kindSelect = screen.getByTestId(
      "notify-form-kind",
    ) as HTMLSelectElement;
    expect(kindSelect.disabled).toBe(true);
  });
});
