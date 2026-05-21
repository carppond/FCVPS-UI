import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy, Download, Upload, Loader2, ArrowLeftFromLine } from "lucide-react";
import Editor, { type OnMount } from "@monaco-editor/react";
import { Button } from "@/components/ui/button";
import { useUIStore } from "@/stores/ui-store";
import { usePipelineSync } from "@/components/pipeline/sync-hook";
import { cn } from "@/lib/cn";

interface YamlPaneProps {
  /** Callback to switch the right rail back to the param panel. */
  onBackToParam?: () => void;
  className?: string;
}

/**
 * Right-rail YAML editor backed by `@monaco-editor/react`.
 *
 *  - Theme: tracks the project's `ui-store.theme` (dark → `vs-dark`,
 *    light → `vs`). When the user picks `system` we resolve it via the
 *    `<html data-theme>` attribute the theme module already applies.
 *  - Toolbar: copy / download / import (upload-file).
 *  - Editor content is wired to `usePipelineSync` so edits round-trip to the
 *    AST via POST /api/pipelines/yaml-to-ast (300 ms debounce).
 */
export function YamlPane({ onBackToParam, className }: YamlPaneProps) {
  const { t } = useTranslation(["pipeline", "common"]);
  const themePref = useUIStore((s) => s.theme);
  const resolvedTheme = useResolvedTheme(themePref);

  const { yaml, setYamlFromEditor, yamlError, isParsing, isSerializing } =
    usePipelineSync();

  const [copied, setCopied] = React.useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(yaml);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // navigator.clipboard may be unavailable in restricted contexts; the UI
      // already shows the YAML so the user can copy manually.
    }
  };

  const handleDownload = () => {
    const blob = new Blob([yaml], { type: "text/yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "pipeline.yaml";
    a.click();
    URL.revokeObjectURL(url);
  };

  const fileInputRef = React.useRef<HTMLInputElement | null>(null);
  const handleImport = () => fileInputRef.current?.click();
  const onFilePicked = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const text = String(reader.result ?? "");
      setYamlFromEditor(text);
    };
    reader.readAsText(file);
    // Reset so re-picking the same file works.
    e.target.value = "";
  };

  const handleEditorMount: OnMount = (editor) => {
    // Slim down chrome to fit the rail. YAML language tokens ship with
    // monaco-editor's basic-languages bundle (no extra registration needed).
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

  return (
    <aside
      data-testid="yaml-pane"
      className={cn(
        "flex h-full w-[28rem] shrink-0 flex-col border-l border-[var(--color-border)]",
        "bg-[var(--color-bg-elevated)]",
        className,
      )}
    >
      <header className="flex items-center justify-between gap-2 border-b border-[var(--color-border)] px-3 py-2">
        <div className="flex items-center gap-2">
          <h2 className="text-[var(--font-size-sm)] font-semibold uppercase tracking-wide text-[var(--color-text-tertiary)]">
            {t("pipeline:yaml_pane.title")}
          </h2>
          {(isParsing || isSerializing) && (
            <Loader2
              className="h-3.5 w-3.5 animate-spin text-[var(--color-text-tertiary)]"
              aria-hidden
            />
          )}
        </div>
        <div className="flex items-center gap-1">
          <Button
            size="sm"
            variant="ghost"
            onClick={handleCopy}
            data-testid="yaml-pane-copy"
            aria-label={t("pipeline:yaml_pane.copy")}
          >
            <Copy className="h-3.5 w-3.5" />
            <span className="ml-1 hidden sm:inline">
              {copied
                ? t("pipeline:yaml_pane.copied")
                : t("pipeline:yaml_pane.copy")}
            </span>
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={handleDownload}
            data-testid="yaml-pane-download"
            aria-label={t("pipeline:yaml_pane.download")}
          >
            <Download className="h-3.5 w-3.5" />
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={handleImport}
            data-testid="yaml-pane-import"
            aria-label={t("pipeline:yaml_pane.import")}
          >
            <Upload className="h-3.5 w-3.5" />
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".yaml,.yml,text/yaml"
            className="hidden"
            onChange={onFilePicked}
            data-testid="yaml-pane-file-input"
          />
          {onBackToParam && (
            <Button
              size="sm"
              variant="ghost"
              onClick={onBackToParam}
              data-testid="yaml-pane-switch-to-param"
              aria-label={t("pipeline:yaml_pane.switch_to_param")}
            >
              <ArrowLeftFromLine className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      </header>

      <div className="relative min-h-0 flex-1">
        <Editor
          height="100%"
          language="yaml"
          value={yaml}
          theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
          onChange={(v) => setYamlFromEditor(v ?? "")}
          onMount={handleEditorMount}
          loading={
            <div className="flex h-full items-center justify-center text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("pipeline:yaml_pane.loading")}
            </div>
          }
          options={{ automaticLayout: true }}
        />
      </div>

      {yamlError && (
        <footer
          role="alert"
          data-testid="yaml-pane-error"
          className="border-t border-[var(--color-border)] bg-[var(--color-error-bg)] px-3 py-2 text-[var(--font-size-xs)] text-[var(--color-error)]"
        >
          {t("pipeline:yaml_pane.sync_error", { message: yamlError })}
        </footer>
      )}
    </aside>
  );
}

/**
 * Resolves the persisted theme preference into a concrete light / dark value
 * the Monaco editor can apply. `system` is read off `<html data-theme>` which
 * the theme module keeps in sync with `prefers-color-scheme`.
 */
function useResolvedTheme(pref: "light" | "dark" | "system"): "light" | "dark" {
  const [resolved, setResolved] = React.useState<"light" | "dark">(() =>
    readDocTheme(),
  );
  React.useEffect(() => {
    if (pref !== "system") {
      setResolved(pref);
      return;
    }
    setResolved(readDocTheme());
    const observer = new MutationObserver(() => setResolved(readDocTheme()));
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, [pref]);
  return resolved;
}

function readDocTheme(): "light" | "dark" {
  if (typeof document === "undefined") return "dark";
  return document.documentElement.getAttribute("data-theme") === "light"
    ? "light"
    : "dark";
}
