import * as React from "react";
import { useTranslation } from "react-i18next";
import Editor, { type OnMount } from "@monaco-editor/react";
import { Eye, RotateCcw, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useMonacoTheme } from "@/components/script/use-monaco-theme";
import { cn } from "@/lib/cn";

/**
 * Go template variables exposed by the backend renderer. Mirrors
 * `internal/notify/templates.go::TemplateData`. The list is rendered as
 * insert-on-click chips beside the editor so users don't need to memorise
 * the variable tree.
 */
export const TEMPLATE_VARIABLES = [
  ".Event.Type",
  ".Event.UserID",
  ".Event.Time",
  ".Resource.Name",
  ".Resource.ID",
  ".Payload",
];

interface TemplateEditorProps {
  /** Current template value. Empty string = inherit the default. */
  value: string;
  onChange: (next: string) => void;
  /** Fires when the user clicks "save". Parent persists. */
  onSave?: (value: string) => Promise<void> | void;
  /** Fires when the user clicks "reset" — parent restores the default. */
  onReset?: () => void;
  /** Sample event used by the preview button (pretty-printed JSON). */
  previewSample?: unknown;
  isSaving?: boolean;
  className?: string;
}

/**
 * Monaco-backed Go-template editor with a side rail of variable chips
 * and a "preview" pane that performs a naive `{{ .Var }}` substitution
 * client-side. The real Go-template render lives on the backend; the
 * client preview is intentionally a sanity check, not a fidelity test.
 */
export function TemplateEditor({
  value,
  onChange,
  onSave,
  onReset,
  previewSample,
  isSaving = false,
  className,
}: TemplateEditorProps) {
  const { t } = useTranslation(["notify", "common"]);
  const theme = useMonacoTheme();
  const editorRef = React.useRef<Parameters<OnMount>[0] | null>(null);

  const [showPreview, setShowPreview] = React.useState(false);

  const handleMount: OnMount = (editor) => {
    editorRef.current = editor;
    editor.updateOptions({
      minimap: { enabled: false },
      lineNumbers: "on",
      scrollBeyondLastLine: false,
      fontSize: 12,
      tabSize: 2,
      wordWrap: "on",
      automaticLayout: true,
    });
  };

  const insertVariable = (variable: string) => {
    const ed = editorRef.current;
    if (!ed) {
      onChange(`${value}{{ ${variable} }}`);
      return;
    }
    const selection = ed.getSelection();
    if (!selection) {
      onChange(`${value}{{ ${variable} }}`);
      return;
    }
    ed.executeEdits("notify-insert-var", [
      {
        range: selection,
        text: `{{ ${variable} }}`,
        forceMoveMarkers: true,
      },
    ]);
    ed.focus();
  };

  const previewText = React.useMemo(() => {
    if (!showPreview) return "";
    return naiveRender(value, previewSample ?? defaultSample());
  }, [showPreview, value, previewSample]);

  return (
    <div className={cn("flex flex-col gap-[var(--spacing-3)]", className)}>
      <header className="flex items-center justify-between gap-[var(--spacing-3)]">
        <div>
          <Label>{t("notify:template.title")}</Label>
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("notify:template.subtitle")}
          </p>
        </div>
        <div className="flex items-center gap-[var(--spacing-2)]">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setShowPreview((p) => !p)}
          >
            <Eye className="h-4 w-4" />
            {showPreview
              ? t("notify:template.hide_preview")
              : t("notify:template.preview")}
          </Button>
          {onReset && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onReset}
              disabled={isSaving}
            >
              <RotateCcw className="h-4 w-4" />
              {t("notify:template.reset")}
            </Button>
          )}
          {onSave && (
            <Button
              type="button"
              size="sm"
              disabled={isSaving}
              onClick={() => void onSave(value)}
            >
              <Save className="h-4 w-4" />
              {isSaving ? t("common:loading") : t("common:actions.save")}
            </Button>
          )}
        </div>
      </header>

      <div className="grid grid-cols-1 gap-[var(--spacing-3)] lg:grid-cols-[1fr_180px]">
        <div className="h-72 overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)]">
          <Editor
            height="100%"
            defaultLanguage="plaintext"
            value={value}
            theme={theme === "dark" ? "vs-dark" : "vs"}
            onChange={(v) => onChange(v ?? "")}
            onMount={handleMount}
            loading={
              <div className="flex h-full items-center justify-center text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("common:loading")}
              </div>
            }
            options={{ automaticLayout: true }}
          />
        </div>

        <aside className="flex flex-col gap-[var(--spacing-2)]">
          <span className="text-[var(--font-size-xs)] font-medium uppercase tracking-wide text-[var(--color-text-tertiary)]">
            {t("notify:template.vars_header")}
          </span>
          <ul className="flex flex-col gap-[var(--spacing-1)]">
            {TEMPLATE_VARIABLES.map((v) => (
              <li key={v}>
                <button
                  type="button"
                  onClick={() => insertVariable(v)}
                  className="w-full rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-[var(--spacing-2)] py-[var(--spacing-1)] text-left font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-hover)]"
                >
                  {`{{ ${v} }}`}
                </button>
              </li>
            ))}
          </ul>
        </aside>
      </div>

      {showPreview && (
        <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-[var(--spacing-3)]">
          <div className="mb-[var(--spacing-2)] text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
            {t("notify:template.preview_header")}
          </div>
          <pre className="whitespace-pre-wrap font-mono text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
            {previewText}
          </pre>
        </div>
      )}
    </div>
  );
}

// ─── Internal helpers ───────────────────────────────────────────────────────

/**
 * Substitutes `{{ .Path.To.Field }}` against a JS object. Not a Go template
 * engine — just enough to give the user visual feedback that variables
 * resolve to something sensible.
 */
function naiveRender(template: string, data: unknown): string {
  return template.replace(/\{\{\s*\.([\w.]+)\s*\}\}/g, (_match, path: string) => {
    const parts = path.split(".");
    let cur: unknown = data;
    for (const part of parts) {
      if (cur && typeof cur === "object" && part in (cur as object)) {
        cur = (cur as Record<string, unknown>)[part];
      } else {
        return `<${path}?>`;
      }
    }
    if (typeof cur === "string") return cur;
    if (typeof cur === "number" || typeof cur === "boolean") return String(cur);
    return JSON.stringify(cur);
  });
}

function defaultSample(): Record<string, unknown> {
  return {
    Event: {
      Type: "node_offline",
      UserID: "u_sample",
      Time: new Date().toISOString(),
    },
    Resource: { Name: "HK-Node-01", ID: "node_abc123" },
    Payload: { reason: "heartbeat timeout" },
  };
}
