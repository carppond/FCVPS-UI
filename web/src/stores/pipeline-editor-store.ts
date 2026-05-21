import { create } from "zustand";
import { newId } from "@/lib/ids";
import type {
  DedupeArgs,
  FilterArgs,
  MapArgs,
  OperatorType,
  OutputArgs,
  Pipeline,
  PipelineOperator,
  RegexRenameArgs,
  RunPipelineResponse,
  SortArgs,
} from "@/types/api";

/**
 * In-memory AST manipulated by the editor. The on-the-wire DTO `Pipeline`
 * stores AST as `ast_json` (string); we keep the parsed array here for the
 * canvas / library to mutate without re-parsing on every render.
 */
export interface PipelineAST {
  /** Operator chain, position is implicit (array index). */
  operators: PipelineOperator[];
}

/**
 * Default args per operator type — kept in one place so both
 * `addOperator` and any future "duplicate" action stay consistent. The
 * parameter forms in T-21 will refine these with zod-validated values.
 */
function defaultArgs(type: OperatorType): PipelineOperator["params"] {
  switch (type) {
    case "filter":
      return { expr: "true" } satisfies FilterArgs;
    case "map":
      return { field: "tag", value: "" } satisfies MapArgs;
    case "sort":
      return { key: "tag", order: "asc" } satisfies SortArgs;
    case "dedupe":
      return { fields: ["server", "port"] } satisfies DedupeArgs;
    case "regex_rename":
      return { pattern: "", replacement: "" } satisfies RegexRenameArgs;
    case "output":
      return { format: "clash" } satisfies OutputArgs;
  }
}

interface PipelineEditorState {
  /** Pipeline metadata + current AST. `pipelineId === null` means "new pipeline". */
  pipelineId: string | null;
  name: string;
  version: number;
  yamlContent: string;
  ast: PipelineAST;
  selectedOperatorId: string | null;
  /** True when ast/name diverge from the last saved snapshot. */
  dirty: boolean;
  /** Latest debug-preview response (T-21 will render this). */
  debugTrace: RunPipelineResponse | null;
}

interface PipelineEditorActions {
  /** Initialise editor state from a server-side `Pipeline`. */
  loadPipeline: (pipeline: Pipeline) => void;
  /** Initialise editor state for a brand-new (unsaved) pipeline. */
  resetForNew: (name?: string) => void;
  /** Update the pipeline display name (marks dirty). */
  setName: (name: string) => void;
  /** Append an operator of the given type to the end of the chain. */
  addOperator: (type: OperatorType, insertIndex?: number) => string;
  /** Remove an operator by id. */
  removeOperator: (id: string) => void;
  /** Reorder operators by index (drag-sort). */
  reorderOperators: (fromIndex: number, toIndex: number) => void;
  /** Select / deselect an operator (id=null clears selection). */
  selectOperator: (id: string | null) => void;
  /** Store a debug-preview response (consumed by T-21 dialog). */
  setDebugTrace: (trace: RunPipelineResponse | null) => void;
  /** Return the current AST + name for PUT /api/pipelines/:id. */
  getPipelineSnapshot: () => {
    name: string;
    ast: PipelineAST;
    version: number;
  };
  /** Mark current state as saved (after a successful PUT). */
  markClean: (next?: { version?: number; yamlContent?: string }) => void;
}

const EMPTY_STATE: PipelineEditorState = {
  pipelineId: null,
  name: "",
  version: 0,
  yamlContent: "",
  ast: { operators: [] },
  selectedOperatorId: null,
  dirty: false,
  debugTrace: null,
};

function parseAst(raw: string): PipelineAST {
  if (!raw) return { operators: [] };
  try {
    const parsed = JSON.parse(raw) as Partial<PipelineAST>;
    if (parsed && Array.isArray(parsed.operators)) {
      return { operators: parsed.operators };
    }
    return { operators: [] };
  } catch {
    // Backend should always emit valid JSON; if not, fall back to empty so
    // the editor stays usable instead of crashing.
    return { operators: [] };
  }
}

function reindex(operators: PipelineOperator[]): PipelineOperator[] {
  return operators.map((op, idx) => ({ ...op, position: idx }));
}

export const usePipelineEditorStore = create<
  PipelineEditorState & PipelineEditorActions
>()((set, get) => ({
  ...EMPTY_STATE,

  loadPipeline: (pipeline) =>
    set({
      pipelineId: pipeline.id,
      name: pipeline.name,
      version: pipeline.version,
      yamlContent: pipeline.yaml_content,
      ast: parseAst(pipeline.ast_json),
      selectedOperatorId: null,
      dirty: false,
      debugTrace: null,
    }),

  resetForNew: (name = "") =>
    set({
      ...EMPTY_STATE,
      name,
      dirty: false,
    }),

  setName: (name) =>
    set((s) => (s.name === name ? s : { ...s, name, dirty: true })),

  addOperator: (type, insertIndex) => {
    const id = newId();
    const op: PipelineOperator = {
      id,
      type,
      enabled: true,
      params: defaultArgs(type),
      position: 0, // re-computed by reindex
    };
    set((s) => {
      const list = [...s.ast.operators];
      if (
        insertIndex === undefined ||
        insertIndex < 0 ||
        insertIndex > list.length
      ) {
        list.push(op);
      } else {
        list.splice(insertIndex, 0, op);
      }
      return {
        ...s,
        ast: { operators: reindex(list) },
        selectedOperatorId: id,
        dirty: true,
      };
    });
    return id;
  },

  removeOperator: (id) =>
    set((s) => {
      const next = s.ast.operators.filter((op) => op.id !== id);
      if (next.length === s.ast.operators.length) return s;
      return {
        ...s,
        ast: { operators: reindex(next) },
        selectedOperatorId:
          s.selectedOperatorId === id ? null : s.selectedOperatorId,
        dirty: true,
      };
    }),

  reorderOperators: (fromIndex, toIndex) =>
    set((s) => {
      const list = [...s.ast.operators];
      if (
        fromIndex === toIndex ||
        fromIndex < 0 ||
        toIndex < 0 ||
        fromIndex >= list.length ||
        toIndex >= list.length
      ) {
        return s;
      }
      const [moved] = list.splice(fromIndex, 1);
      list.splice(toIndex, 0, moved);
      return {
        ...s,
        ast: { operators: reindex(list) },
        dirty: true,
      };
    }),

  selectOperator: (id) =>
    set((s) =>
      s.selectedOperatorId === id ? s : { ...s, selectedOperatorId: id },
    ),

  setDebugTrace: (trace) => set({ debugTrace: trace }),

  getPipelineSnapshot: () => {
    const s = get();
    return { name: s.name, ast: s.ast, version: s.version };
  },

  markClean: (next) =>
    set((s) => ({
      ...s,
      dirty: false,
      version: next?.version ?? s.version,
      yamlContent: next?.yamlContent ?? s.yamlContent,
    })),
}));
