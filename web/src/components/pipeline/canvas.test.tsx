import * as React from "react";
import { describe, it, expect, beforeEach, beforeAll } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { DndContext } from "@dnd-kit/core";
import { PipelineCanvas, resolveCanvasDragEnd } from "./canvas";
import { libraryDraggableId } from "./operator-library";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import "@/lib/i18n";
import i18n from "@/lib/i18n";

beforeAll(async () => {
  await i18n.changeLanguage("en");
});

// Reset the editor store between tests so state doesn't bleed across cases.
beforeEach(() => {
  usePipelineEditorStore.getState().resetForNew("test pipeline");
});

function Wrapper() {
  return (
    <DndContext>
      <PipelineCanvas />
    </DndContext>
  );
}

describe("PipelineCanvas", () => {
  it("renders an empty-state when no operators exist", () => {
    render(<Wrapper />);
    expect(screen.getByTestId("canvas-empty")).toBeInTheDocument();
  });

  it("renders a sortable list when operators exist in the store", () => {
    act(() => {
      usePipelineEditorStore.getState().addOperator("filter");
      usePipelineEditorStore.getState().addOperator("sort");
    });
    render(<Wrapper />);
    expect(screen.getByTestId("canvas-operator-list")).toBeInTheDocument();
    expect(screen.queryByTestId("canvas-empty")).not.toBeInTheDocument();

    // Both operator nodes should be rendered with their data-operator-type.
    const nodes = document.querySelectorAll<HTMLDivElement>(
      "[data-operator-type]",
    );
    const types = Array.from(nodes).map((n) => n.dataset.operatorType);
    expect(types).toEqual(["filter", "sort"]);
  });

  it("removes an operator when the trash button is clicked", () => {
    let dedupeId = "";
    act(() => {
      usePipelineEditorStore.getState().addOperator("filter");
      dedupeId = usePipelineEditorStore.getState().addOperator("dedupe");
    });
    render(<Wrapper />);
    expect(
      document.querySelector(`[data-operator-id="${dedupeId}"]`),
    ).toBeInTheDocument();

    const removeBtn = screen.getByTestId(`operator-node-remove-${dedupeId}`);
    fireEvent.click(removeBtn);

    expect(
      document.querySelector(`[data-operator-id="${dedupeId}"]`),
    ).not.toBeInTheDocument();
    // Only one operator should remain (the filter).
    expect(
      usePipelineEditorStore.getState().ast.operators,
    ).toHaveLength(1);
  });

  it("selects an operator when its body is clicked", () => {
    let filterId = "";
    act(() => {
      filterId = usePipelineEditorStore.getState().addOperator("filter");
      // addOperator auto-selects; clear so the click drives selection.
      usePipelineEditorStore.getState().selectOperator(null);
    });
    render(<Wrapper />);
    expect(
      usePipelineEditorStore.getState().selectedOperatorId,
    ).toBeNull();

    fireEvent.click(screen.getByTestId(`operator-node-body-${filterId}`));
    expect(
      usePipelineEditorStore.getState().selectedOperatorId,
    ).toBe(filterId);
  });
});

describe("resolveCanvasDragEnd", () => {
  it("adds a new operator when a library draggable is dropped on the canvas", () => {
    const store = usePipelineEditorStore.getState();
    expect(store.ast.operators).toHaveLength(0);

    const resolution = resolveCanvasDragEnd({
      activeId: libraryDraggableId("filter"),
      overId: "pipeline-canvas-drop",
      operators: store.ast.operators,
      store: {
        addOperator: store.addOperator,
        reorderOperators: store.reorderOperators,
      },
    });

    expect(resolution.kind).toBe("add");
    expect(resolution.newOperatorId).toBeTruthy();
    const next = usePipelineEditorStore.getState().ast.operators;
    expect(next).toHaveLength(1);
    expect(next[0].type).toBe("filter");
  });

  it("inserts after the target operator when a library card is dropped on an existing node", () => {
    act(() => {
      usePipelineEditorStore.getState().addOperator("filter");
      usePipelineEditorStore.getState().addOperator("sort");
    });
    const before = usePipelineEditorStore.getState().ast.operators;
    expect(before.map((o) => o.type)).toEqual(["filter", "sort"]);

    const store = usePipelineEditorStore.getState();
    resolveCanvasDragEnd({
      activeId: libraryDraggableId("output"),
      overId: before[0].id, // drop on the "filter" node
      operators: store.ast.operators,
      store: {
        addOperator: store.addOperator,
        reorderOperators: store.reorderOperators,
      },
    });

    const after = usePipelineEditorStore.getState().ast.operators;
    expect(after.map((o) => o.type)).toEqual(["filter", "output", "sort"]);
  });

  it("reorders operators when an existing node is dragged onto another", () => {
    act(() => {
      usePipelineEditorStore.getState().addOperator("filter");
      usePipelineEditorStore.getState().addOperator("sort");
      usePipelineEditorStore.getState().addOperator("output");
    });
    const ops = usePipelineEditorStore.getState().ast.operators;
    expect(ops.map((o) => o.type)).toEqual(["filter", "sort", "output"]);

    // Move "filter" → onto "output" (i.e., to the end).
    const store = usePipelineEditorStore.getState();
    const resolution = resolveCanvasDragEnd({
      activeId: ops[0].id,
      overId: ops[2].id,
      operators: store.ast.operators,
      store: {
        addOperator: store.addOperator,
        reorderOperators: store.reorderOperators,
      },
    });
    expect(resolution.kind).toBe("reorder");

    const after = usePipelineEditorStore.getState().ast.operators;
    expect(after.map((o) => o.type)).toEqual(["sort", "output", "filter"]);
    // Positions should be re-indexed after reorder.
    expect(after.map((o) => o.position)).toEqual([0, 1, 2]);
  });

  it("returns noop when dropped outside any droppable", () => {
    const store = usePipelineEditorStore.getState();
    const resolution = resolveCanvasDragEnd({
      activeId: libraryDraggableId("filter"),
      overId: null,
      operators: store.ast.operators,
      store: {
        addOperator: store.addOperator,
        reorderOperators: store.reorderOperators,
      },
    });
    expect(resolution.kind).toBe("noop");
    expect(usePipelineEditorStore.getState().ast.operators).toHaveLength(0);
  });

  it("returns noop when reorder source and target are the same", () => {
    act(() => {
      usePipelineEditorStore.getState().addOperator("filter");
    });
    const ops = usePipelineEditorStore.getState().ast.operators;
    const store = usePipelineEditorStore.getState();
    const resolution = resolveCanvasDragEnd({
      activeId: ops[0].id,
      overId: ops[0].id,
      operators: store.ast.operators,
      store: {
        addOperator: store.addOperator,
        reorderOperators: store.reorderOperators,
      },
    });
    expect(resolution.kind).toBe("noop");
  });
});
