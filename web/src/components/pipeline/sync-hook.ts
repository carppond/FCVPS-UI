import * as React from "react";
import { useParseYaml, useToYaml } from "@/api/pipeline";
import { usePipelineEditorStore, type PipelineAST } from "@/stores/pipeline-editor-store";

const DEFAULT_YAML_DEBOUNCE_MS = 300;

/** Public surface of `usePipelineSync`. */
export interface UsePipelineSyncResult {
  /** YAML text rendered by `<YamlPane />`. Mirrors the store AST. */
  yaml: string;
  /** Called by Monaco when the user types into the editor. */
  setYamlFromEditor: (next: string) => void;
  /** Last YAML→AST parse error message (null when valid). */
  yamlError: string | null;
  /** True while a debounced parse is in flight. */
  isParsing: boolean;
  /** True while an AST→YAML round-trip is in flight. */
  isSerializing: boolean;
}

/**
 * Tracks which side of the GUI ↔ YAML bridge is the source of truth for the
 * next round-trip. The hook only kicks off a serialize / parse when the
 * counterpart has *not* already triggered the change, avoiding infinite loops:
 *
 *   - user types YAML  → ref = "yaml" → schedule parse → on apply set ref = "ast"
 *   - canvas mutates  → ref != "yaml" → schedule serialize
 */
type SyncSource = "ast" | "yaml";

interface UsePipelineSyncOptions {
  /** YAML→AST debounce window (ms). Defaults to 300 per task spec. */
  yamlDebounceMs?: number;
}

/**
 * Bidirectional GUI ↔ YAML synchronization hook.
 *
 *  - Subscribes to `pipelineEditorStore.ast` and serialises via POST
 *    /api/pipelines/ast-to-yaml whenever the AST changes from a GUI edit.
 *  - When the YAML pane edits the text, debounces (default 300 ms) then calls
 *    POST /api/pipelines/yaml-to-ast and pushes the parsed AST back into the
 *    store via `replaceAst`.
 *  - A `sourceRef` flag tells the hook which side initiated the change so a
 *    round-trip cannot trigger an infinite loop.
 */
export function usePipelineSync(
  options: UsePipelineSyncOptions = {},
): UsePipelineSyncResult {
  const debounceMs = options.yamlDebounceMs ?? DEFAULT_YAML_DEBOUNCE_MS;

  const ast = usePipelineEditorStore((s) => s.ast);
  const initialYaml = usePipelineEditorStore((s) => s.yamlContent);
  const replaceAst = usePipelineEditorStore((s) => s.replaceAst);

  const parseMutation = useParseYaml();
  const toYamlMutation = useToYaml();

  const [yaml, setYaml] = React.useState<string>(initialYaml ?? "");
  const [yamlError, setYamlError] = React.useState<string | null>(null);

  const sourceRef = React.useRef<SyncSource>("ast");
  const timerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  // ── AST → YAML: triggered when the canvas / param panel mutates the AST.
  React.useEffect(() => {
    if (sourceRef.current === "yaml") {
      // Skip — this change originated from the YAML pane; serialising would
      // either no-op or fight the user's pending keystrokes.
      sourceRef.current = "ast";
      return;
    }
    let cancelled = false;
    const astJson = JSON.stringify(ast);
    toYamlMutation
      .mutateAsync(astJson)
      .then((resp) => {
        if (cancelled) return;
        setYaml(resp.yaml_content);
        setYamlError(null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setYamlError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
    // The mutation hook identity is stable; depend only on the AST to avoid
    // re-firing on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ast]);

  // ── YAML → AST: triggered by user input in the Monaco editor.
  const setYamlFromEditor = React.useCallback(
    (next: string) => {
      sourceRef.current = "yaml";
      setYaml(next);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
        parseMutation
          .mutateAsync(next)
          .then((resp) => {
            try {
              const parsed = JSON.parse(resp.ast_json) as PipelineAST;
              if (parsed && Array.isArray(parsed.operators)) {
                // Mark the upcoming AST→YAML effect as triggered-by-yaml so it
                // doesn't bounce the freshly-parsed YAML back at us.
                sourceRef.current = "yaml";
                replaceAst(parsed);
                setYamlError(null);
              } else {
                setYamlError("Invalid AST shape");
              }
            } catch (err) {
              setYamlError(err instanceof Error ? err.message : String(err));
            }
          })
          .catch((err: unknown) => {
            setYamlError(err instanceof Error ? err.message : String(err));
          });
      }, debounceMs);
    },
    [debounceMs, parseMutation, replaceAst],
  );

  // Cleanup any pending debounce on unmount so it doesn't fire post-teardown.
  React.useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  return {
    yaml,
    setYamlFromEditor,
    yamlError,
    isParsing: parseMutation.isPending,
    isSerializing: toYamlMutation.isPending,
  };
}
