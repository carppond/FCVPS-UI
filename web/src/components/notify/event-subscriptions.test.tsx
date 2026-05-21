import * as React from "react";
import { describe, it, expect, beforeAll, beforeEach, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import i18n from "@/lib/i18n";
import notifyEn from "@/locales/en/notify.json";

import { EventSubscriptions } from "./event-subscriptions";
import type { NotificationChannel } from "@/types/api";

const saveMock = vi.fn().mockResolvedValue([]);

vi.mock("@/api/notify", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/notify")>();
  return {
    ...actual,
    useSaveSubscriptionMatrix: () => ({
      mutateAsync: saveMock,
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
  saveMock.mockClear();
});

function fakeChannel(
  id: string,
  name: string,
  eventTypes: NotificationChannel["event_types"] = [],
): NotificationChannel {
  return {
    id,
    user_id: "u",
    kind: "telegram",
    name,
    config: { bot_token: "x", chat_id: "y" } as never,
    event_types: eventTypes,
    enabled: true,
    created_at: 0,
    updated_at: 0,
  };
}

describe("EventSubscriptions", () => {
  it("renders empty state when no channels are configured", () => {
    renderWithClient(<EventSubscriptions channels={[]} />);
    expect(screen.getByTestId("notify-matrix-empty")).toBeInTheDocument();
  });

  it("renders one column per channel and one row per event", () => {
    renderWithClient(
      <EventSubscriptions
        channels={[fakeChannel("a", "Alpha"), fakeChannel("b", "Beta")]}
      />,
    );

    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
    // 7 rows × 2 channels = 14 cells
    expect(
      screen.getAllByTestId(/notify-matrix-cell-/).length,
    ).toBeGreaterThanOrEqual(14);
  });

  it("toggles a cell on click and reflects the staged change", () => {
    renderWithClient(
      <EventSubscriptions channels={[fakeChannel("a", "Alpha")]} />,
    );

    const cell = screen.getByTestId(
      "notify-matrix-cell-node_offline-a",
    ) as HTMLInputElement;
    expect(cell.checked).toBe(false);

    fireEvent.click(cell);
    expect(cell.checked).toBe(true);
  });

  it("emits the staged matrix to useSaveSubscriptionMatrix on save", async () => {
    renderWithClient(
      <EventSubscriptions
        channels={[fakeChannel("a", "Alpha", ["backup_completed"])]}
      />,
    );

    // Toggle node_offline ON; backup_completed stays ON.
    const cell = screen.getByTestId(
      "notify-matrix-cell-node_offline-a",
    ) as HTMLInputElement;
    fireEvent.click(cell);

    const saveBtn = screen.getByTestId("notify-matrix-save");
    expect(saveBtn).not.toBeDisabled();

    await act(async () => {
      fireEvent.click(saveBtn);
    });

    expect(saveMock).toHaveBeenCalledTimes(1);
    const arg = saveMock.mock.calls[0][0];
    expect(arg).toHaveLength(1);
    expect(arg[0].id).toBe("a");
    expect(new Set(arg[0].event_types)).toEqual(
      new Set(["backup_completed", "node_offline"]),
    );
  });

  it("disables save when no cell differs from the persisted matrix", () => {
    renderWithClient(
      <EventSubscriptions
        channels={[fakeChannel("a", "Alpha", ["backup_completed"])]}
      />,
    );

    const saveBtn = screen.getByTestId("notify-matrix-save");
    expect(saveBtn).toBeDisabled();
  });

  it("disables cells for channels that are not enabled", () => {
    renderWithClient(
      <EventSubscriptions
        channels={[
          {
            ...fakeChannel("a", "Alpha"),
            enabled: false,
          },
        ]}
      />,
    );

    const cell = screen.getByTestId(
      "notify-matrix-cell-node_offline-a",
    ) as HTMLInputElement;
    expect(cell.disabled).toBe(true);
  });
});
