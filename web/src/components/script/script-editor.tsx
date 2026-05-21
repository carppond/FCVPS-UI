import { useTranslation } from "react-i18next";
import Editor, { type OnMount } from "@monaco-editor/react";
import { useMonacoTheme } from "@/components/script/use-monaco-theme";

interface ScriptEditorProps {
  value: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  /** Optional id for the editor `data-testid` attribute. */
  testId?: string;
}

/**
 * ScriptEditor wraps `@monaco-editor/react` with the project's theme
 * conventions:
 *
 *   - Theme tracks the global theme preference (light / dark / system) via
 *     useMonacoTheme so the editor flips alongside the rest of the chrome.
 *   - Chrome is slimmed (no minimap, word-wrap on) so the editor fits the
 *     main column without competing with the test panel for horizontal
 *     space.
 *   - JavaScript language tokens ship with monaco-editor's basic-languages
 *     bundle — no extra registration needed.
 */
export function ScriptEditor({
  value,
  onChange,
  readOnly = false,
  testId,
}: ScriptEditorProps) {
  const { t } = useTranslation("script");
  const theme = useMonacoTheme();

  const handleMount: OnMount = (editor) => {
    editor.updateOptions({
      minimap: { enabled: false },
      lineNumbers: "on",
      scrollBeyondLastLine: false,
      fontSize: 12,
      tabSize: 2,
      wordWrap: "on",
      automaticLayout: true,
      readOnly,
    });
  };

  return (
    <div className="h-full w-full" data-testid={testId}>
      <Editor
        height="100%"
        language="javascript"
        value={value}
        theme={theme === "dark" ? "vs-dark" : "vs"}
        onChange={(v) => onChange(v ?? "")}
        onMount={handleMount}
        loading={
          <div className="flex h-full items-center justify-center text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("editor.loading")}
          </div>
        }
        options={{ automaticLayout: true, readOnly }}
      />
    </div>
  );
}
