import { useTranslation } from "react-i18next";
import Editor, { type OnMount } from "@monaco-editor/react";
import { Loader2, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMonacoTheme } from "@/components/script/use-monaco-theme";
import type { ScriptTestResult } from "@/api/script";

interface ScriptTestPanelProps {
  /** JSON-string-form of the test input. Component owns the textarea. */
  input: string;
  onInputChange: (input: string) => void;
  result?: ScriptTestResult;
  isRunning: boolean;
  onRun: () => void;
  /** Local validation error (e.g. invalid JSON input). */
  localError?: string;
}

/**
 * ScriptTestPanel is the right rail of the editor page. Layout:
 *
 *   - Top: JSON input (Monaco JSON editor) + Run button.
 *   - Middle: output area (read-only JSON Monaco).
 *   - Bottom: logs list + error box (when present).
 *
 * Design rules: no hex / no inline px / use design tokens for every
 * dimension and colour.
 */
export function ScriptTestPanel({
  input,
  onInputChange,
  result,
  isRunning,
  onRun,
  localError,
}: ScriptTestPanelProps) {
  const { t } = useTranslation(["script", "common"]);
  const theme = useMonacoTheme();

  const editorMount: OnMount = (editor) => {
    editor.updateOptions({
      minimap: { enabled: false },
      lineNumbers: "off",
      scrollBeyondLastLine: false,
      fontSize: 12,
      tabSize: 2,
      wordWrap: "on",
      automaticLayout: true,
    });
  };

  return (
    <aside
      data-testid="script-test-panel"
      className="flex h-full w-[24rem] shrink-0 flex-col border-l border-[var(--color-border)] bg-[var(--color-bg-elevated)]"
    >
      <header className="flex items-center justify-between gap-2 border-b border-[var(--color-border)] px-3 py-2">
        <h2 className="text-[var(--font-size-sm)] font-semibold uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("script:test_panel.title")}
        </h2>
        <Button
          size="sm"
          onClick={onRun}
          disabled={isRunning}
          data-testid="script-test-run"
        >
          {isRunning ? (
            <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
          ) : (
            <Play className="mr-1 h-3.5 w-3.5" />
          )}
          {isRunning ? t("script:editor.running") : t("script:editor.run_test")}
        </Button>
      </header>

      <section className="flex min-h-[10rem] flex-col border-b border-[var(--color-border)]">
        <p className="px-3 pb-1 pt-2 text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("script:test_panel.input_label")}
        </p>
        <div className="min-h-0 flex-1">
          <Editor
            height="100%"
            language="json"
            value={input}
            theme={theme === "dark" ? "vs-dark" : "vs"}
            onChange={(v) => onInputChange(v ?? "")}
            onMount={editorMount}
            options={{ automaticLayout: true }}
          />
        </div>
        {localError && (
          <p
            role="alert"
            className="border-t border-[var(--color-border)] bg-[var(--color-error-bg)] px-3 py-1 text-[var(--font-size-xs)] text-[var(--color-error)]"
          >
            {localError}
          </p>
        )}
      </section>

      <section className="flex min-h-[8rem] flex-col border-b border-[var(--color-border)]">
        <p className="flex items-baseline justify-between px-3 pb-1 pt-2">
          <span className="text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
            {t("script:test_panel.output_label")}
          </span>
          {result && (
            <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("script:test_panel.duration", { ms: result.duration_ms })}
            </span>
          )}
        </p>
        <div className="min-h-0 flex-1">
          <Editor
            height="100%"
            language="json"
            value={result?.output ?? ""}
            theme={theme === "dark" ? "vs-dark" : "vs"}
            onMount={editorMount}
            options={{ automaticLayout: true, readOnly: true }}
          />
        </div>
      </section>

      <section className="flex flex-1 flex-col">
        <p className="px-3 pb-1 pt-2 text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("script:test_panel.logs_label")}
        </p>
        <div className="min-h-0 flex-1 overflow-auto px-3 pb-2 text-[var(--font-size-xs)] font-mono text-[var(--color-text-secondary)]">
          {result?.logs && result.logs.length > 0 ? (
            <ul className="flex flex-col gap-0.5">
              {result.logs.map((line, i) => (
                <li key={i}>{line}</li>
              ))}
            </ul>
          ) : (
            <p className="text-[var(--color-text-tertiary)]">
              {t("script:test_panel.logs_empty")}
            </p>
          )}
        </div>
      </section>

      {result?.error && (
        <section
          role="alert"
          className="border-t border-[var(--color-border)] bg-[var(--color-error-bg)] px-3 py-2 text-[var(--font-size-xs)] text-[var(--color-error)]"
        >
          <p className="font-semibold uppercase tracking-wide">
            {t("script:test_panel.error_label")}
          </p>
          <p className="mt-0.5 break-all">{result.error}</p>
        </section>
      )}
    </aside>
  );
}
