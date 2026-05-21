import { describe, it, expect, beforeEach, vi } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

// ── Mocks for the API mutations used by `usePipelineSync` ────────────────────
//
// We replace the @tanstack/react-query mutations so the hook can run without
// a network stack. Each mock exposes a controllable async function so the test
// can wait for the next microtask before asserting.

const toYamlMock = vi.fn<
  (astJson: string) => Promise<{ yaml_content: string }>
>();
const parseYamlMock = vi.fn<
  (yaml: string) => Promise<{ ast_json: string }>
>();

vi.mock("@/api/pipeline", () => ({
  useToYaml: () => ({
    mutateAsync: (astJson: string) => toYamlMock(astJson),
    isPending: false,
  }),
  useParseYaml: () => ({
    mutateAsync: (yaml: string) => parseYamlMock(yaml),
    isPending: false,
  }),
}));

import { usePipelineSync } from "./sync-hook";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";

beforeEach(() => {
  toYamlMock.mockReset();
  parseYamlMock.mockReset();
  usePipelineEditorStore.getState().resetForNew("sync test");
});

describe("usePipelineSync", () => {
  it("serialises the AST when the store changes (ast → yaml)", async () => {
    toYamlMock.mockResolvedValue({ yaml_content: "operators: []\n" });

    const { result } = renderHook(() => usePipelineSync());

    // Initial mount fires one serialise pass with the empty AST.
    await waitFor(() => {
      expect(toYamlMock).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(result.current.yaml).toBe("operators: []\n");
    });

    // Now mutate the store and assert the next serialise carries the change.
    toYamlMock.mockResolvedValue({
      yaml_content: "operators:\n  - type: filter\n",
    });
    act(() => {
      usePipelineEditorStore.getState().addOperator("filter");
    });
    await waitFor(() => {
      expect(toYamlMock).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(result.current.yaml).toBe("operators:\n  - type: filter\n");
    });
  });

  it("debounces parse calls when the user edits YAML", async () => {
    toYamlMock.mockResolvedValue({ yaml_content: "operators: []\n" });
    parseYamlMock.mockResolvedValue({
      ast_json: JSON.stringify({ operators: [] }),
    });

    const { result } = renderHook(() =>
      usePipelineSync({ yamlDebounceMs: 25 }),
    );

    act(() => result.current.setYamlFromEditor("operators: []\n# edit"));

    // Should NOT have parsed yet — still inside debounce window.
    expect(parseYamlMock).not.toHaveBeenCalled();

    await waitFor(
      () => {
        expect(parseYamlMock).toHaveBeenCalledTimes(1);
      },
      { timeout: 200 },
    );
    expect(parseYamlMock).toHaveBeenCalledWith("operators: []\n# edit");
  });

  it("surfaces YAML→AST parse errors on the `yamlError` field", async () => {
    toYamlMock.mockResolvedValue({ yaml_content: "operators: []\n" });
    parseYamlMock.mockRejectedValueOnce(new Error("bad yaml"));

    const { result } = renderHook(() =>
      usePipelineSync({ yamlDebounceMs: 5 }),
    );

    act(() => result.current.setYamlFromEditor("not yaml :::"));

    await waitFor(() => {
      expect(result.current.yamlError).toBe("bad yaml");
    });
  });

  it("pushes the parsed AST back into the store via replaceAst", async () => {
    toYamlMock.mockResolvedValue({ yaml_content: "operators: []\n" });
    const parsedAst = {
      operators: [
        {
          id: "from-yaml",
          type: "filter" as const,
          enabled: true,
          params: { expr: "true" },
          position: 0,
        },
      ],
    };
    parseYamlMock.mockResolvedValueOnce({
      ast_json: JSON.stringify(parsedAst),
    });

    const { result } = renderHook(() =>
      usePipelineSync({ yamlDebounceMs: 5 }),
    );

    act(() =>
      result.current.setYamlFromEditor(
        "operators:\n  - id: from-yaml\n    type: filter\n    enabled: true\n    params:\n      expr: 'true'\n    position: 0\n",
      ),
    );

    await waitFor(() => {
      const ops = usePipelineEditorStore.getState().ast.operators;
      expect(ops).toHaveLength(1);
      expect(ops[0].id).toBe("from-yaml");
    });
  });
});
