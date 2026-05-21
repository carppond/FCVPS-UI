import * as React from "react";
import { describe, it, expect, beforeEach, beforeAll, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { ParamPanel } from "./param-panel";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import "@/lib/i18n";
import i18n from "@/lib/i18n";

beforeAll(async () => {
  await i18n.changeLanguage("en");
});

beforeEach(() => {
  usePipelineEditorStore.getState().resetForNew("test pipeline");
});

describe("ParamPanel", () => {
  it("renders the no-selection empty state when no operator is selected", () => {
    render(<ParamPanel />);
    expect(
      screen.getByText(i18n.t("pipeline:param_form.common.no_selection_title")),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("param-panel-header")).not.toBeInTheDocument();
  });

  it("renders the header with operator name + type badge once an operator is selected", () => {
    let id = "";
    act(() => {
      id = usePipelineEditorStore.getState().addOperator("filter");
      usePipelineEditorStore.getState().selectOperator(id);
    });
    render(<ParamPanel />);
    const header = screen.getByTestId("param-panel-header");
    expect(header).toBeInTheDocument();
    expect(screen.getByTestId("param-panel-type-badge")).toHaveTextContent(
      "filter",
    );
    expect(screen.getByTestId("filter-expr")).toBeInTheDocument();
  });

  it("propagates filter expr changes back to the store and flips dirty", () => {
    let id = "";
    act(() => {
      id = usePipelineEditorStore.getState().addOperator("filter");
      usePipelineEditorStore.getState().selectOperator(id);
      // addOperator already marks dirty; clear it so we can assert the change.
      usePipelineEditorStore.getState().markClean();
    });
    render(<ParamPanel />);
    const input = screen.getByTestId("filter-expr") as HTMLInputElement;
    fireEvent.change(input, {
      target: { value: 'protocol == "vmess"' },
    });
    const op = usePipelineEditorStore
      .getState()
      .ast.operators.find((o) => o.id === id);
    expect(op?.params).toEqual({ expr: 'protocol == "vmess"' });
    expect(usePipelineEditorStore.getState().dirty).toBe(true);
  });

  it("renders the sort form when a sort operator is selected", () => {
    act(() => {
      const id = usePipelineEditorStore.getState().addOperator("sort");
      usePipelineEditorStore.getState().selectOperator(id);
    });
    render(<ParamPanel />);
    expect(screen.getByTestId("sort-key")).toBeInTheDocument();
    expect(screen.getByTestId("sort-order")).toBeInTheDocument();
  });

  it("renders the output form with format + max-nodes controls", () => {
    act(() => {
      const id = usePipelineEditorStore.getState().addOperator("output");
      usePipelineEditorStore.getState().selectOperator(id);
    });
    render(<ParamPanel />);
    expect(screen.getByTestId("output-format")).toBeInTheDocument();
    expect(screen.getByTestId("output-max-nodes")).toBeInTheDocument();
  });

  it("renders the regex-rename form and flags an invalid pattern", () => {
    let id = "";
    act(() => {
      id = usePipelineEditorStore.getState().addOperator("regex_rename");
      usePipelineEditorStore.getState().selectOperator(id);
    });
    render(<ParamPanel />);
    const pat = screen.getByTestId("rename-pattern") as HTMLInputElement;
    fireEvent.change(pat, { target: { value: "[unclosed" } });
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      usePipelineEditorStore
        .getState()
        .ast.operators.find((o) => o.id === id)?.params,
    ).toMatchObject({ pattern: "[unclosed" });
  });

  it("deletes the operator when the delete button is clicked", () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    let id = "";
    act(() => {
      id = usePipelineEditorStore.getState().addOperator("dedupe");
      usePipelineEditorStore.getState().selectOperator(id);
    });
    render(<ParamPanel />);
    fireEvent.click(screen.getByTestId("param-panel-delete"));
    expect(
      usePipelineEditorStore.getState().ast.operators.find((o) => o.id === id),
    ).toBeUndefined();
    confirmSpy.mockRestore();
  });

  it("does not delete the operator when the confirm dialog is dismissed", () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    act(() => {
      const id = usePipelineEditorStore.getState().addOperator("dedupe");
      usePipelineEditorStore.getState().selectOperator(id);
    });
    render(<ParamPanel />);
    fireEvent.click(screen.getByTestId("param-panel-delete"));
    expect(usePipelineEditorStore.getState().ast.operators.length).toBe(1);
    confirmSpy.mockRestore();
  });
});
