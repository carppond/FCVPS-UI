import * as React from "react";
import {
  describe,
  it,
  expect,
  beforeEach,
  beforeAll,
  vi,
} from "vitest";
import {
  render,
  screen,
  fireEvent,
  act,
  waitFor,
} from "@testing-library/react";

// Mock the runPreview mutation so we don't need a network stack.
const runPreviewMock = vi.fn();

vi.mock("@/api/pipeline", () => ({
  useRunPreview: () => ({
    mutateAsync: (...args: unknown[]) => runPreviewMock(...args),
    isPending: false,
  }),
}));

// Mock the api-error hook so we don't pull in i18n-error namespaces.
vi.mock("@/hooks/use-api-error", () => ({
  useApiError: () => ({ handle: () => undefined, format: () => "" }),
}));

import { PreviewPane } from "./preview-pane";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import "@/lib/i18n";
import i18n from "@/lib/i18n";

beforeAll(async () => {
  await i18n.changeLanguage("en");
});

beforeEach(() => {
  runPreviewMock.mockReset();
  usePipelineEditorStore.getState().resetForNew("preview-test");
});

describe("PreviewPane", () => {
  it("shows the empty-state until a preview has been run", () => {
    render(
      <PreviewPane open onOpenChange={() => undefined} pipelineId="p-1" />,
    );
    expect(
      screen.getByText(i18n.t("pipeline:preview.no_result_title")),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("preview-result")).not.toBeInTheDocument();
  });

  it("disables the Run button until a subscription id is entered", () => {
    render(
      <PreviewPane open onOpenChange={() => undefined} pipelineId="p-1" />,
    );
    const btn = screen.getByTestId("preview-run") as HTMLButtonElement;
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByTestId("preview-subscription"), {
      target: { value: "sub-abc" },
    });
    expect(btn).not.toBeDisabled();
  });

  it("invokes runPreview with the entered subscription id and renders the trace", async () => {
    runPreviewMock.mockResolvedValue({
      total_ms: 12,
      output_count: 3,
      steps: [
        {
          operator: "op-1",
          input_count: 5,
          output_count: 3,
          added: [],
          removed: ["A", "B"],
          modified: [],
        },
      ],
    });

    // Seed an operator so the renderer can resolve op-1 → operator meta.
    act(() => {
      const id = usePipelineEditorStore.getState().addOperator("filter");
      // Re-key the seeded operator to match the mocked step.
      const state = usePipelineEditorStore.getState();
      const op = state.ast.operators.find((o) => o.id === id);
      if (op) {
        state.updateOperatorParams(op.id, op.params);
        // Replace AST with one whose id matches "op-1".
        state.replaceAst({
          operators: [{ ...op, id: "op-1" }],
        });
      }
    });

    render(
      <PreviewPane open onOpenChange={() => undefined} pipelineId="pipe-7" />,
    );
    fireEvent.change(screen.getByTestId("preview-subscription"), {
      target: { value: "sub-abc" },
    });
    fireEvent.click(screen.getByTestId("preview-run"));

    await waitFor(() => {
      expect(runPreviewMock).toHaveBeenCalledWith({
        id: "pipe-7",
        payload: { subscription_id: "sub-abc", debug: true },
      });
    });
    await waitFor(() => {
      expect(screen.getByTestId("preview-result")).toBeInTheDocument();
    });
    expect(screen.getByTestId("preview-step-0")).toBeInTheDocument();
    // Removed list shows up (count = 2)
    expect(screen.getByText(/Removed \(2\)/i)).toBeInTheDocument();
  });

  it("renders 'no_steps' when the response has no steps array", async () => {
    runPreviewMock.mockResolvedValue({
      total_ms: 3,
      output_count: 0,
    });
    render(
      <PreviewPane open onOpenChange={() => undefined} pipelineId="p-1" />,
    );
    fireEvent.change(screen.getByTestId("preview-subscription"), {
      target: { value: "sub-1" },
    });
    fireEvent.click(screen.getByTestId("preview-run"));
    await waitFor(() => {
      expect(
        screen.getByText(i18n.t("pipeline:preview.no_steps")),
      ).toBeInTheDocument();
    });
  });
});
