import * as React from "react";
import { useTranslation } from "react-i18next";
import { UploadCloud, FileText } from "lucide-react";
import { cn } from "@/lib/cn";
import { formatBytes } from "@/lib/format";

const MAX_FILE_BYTES = 5 * 1024 * 1024; // matches internal/handler maxUploadBytes guard

interface SubUploadProps {
  file: File | null;
  onChange: (file: File | null) => void;
  /** Optional explicit error string to surface beneath the dropzone (i18n already applied). */
  error?: string;
  className?: string;
}

/**
 * Drag-and-drop file picker for YAML / Base64 subscription uploads.
 *
 * Wraps a hidden <input type="file"> so the dropzone is fully keyboard-
 * accessible (clicking or pressing Enter opens the OS file dialog).
 */
export function SubUpload({ file, onChange, error, className }: SubUploadProps) {
  const { t } = useTranslation(["subscription"]);
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const [hover, setHover] = React.useState(false);
  const [localError, setLocalError] = React.useState<string | undefined>();

  const visibleError = error ?? localError;

  const validate = (f: File): boolean => {
    if (f.size > MAX_FILE_BYTES) {
      setLocalError(t("subscription:upload.max_size"));
      return false;
    }
    setLocalError(undefined);
    return true;
  };

  const handleFiles = (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const f = files[0];
    if (!validate(f)) return;
    onChange(f);
  };

  const onDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setHover(false);
    handleFiles(e.dataTransfer.files);
  };

  const onDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setHover(true);
  };

  const onClick = () => inputRef.current?.click();
  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onClick();
    }
  };

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <div
        role="button"
        tabIndex={0}
        onClick={onClick}
        onKeyDown={onKeyDown}
        onDrop={onDrop}
        onDragOver={onDragOver}
        onDragLeave={() => setHover(false)}
        className={cn(
          "flex flex-col items-center justify-center gap-3",
          "rounded-[var(--radius-lg)] border border-dashed",
          "px-6 py-12 text-center transition-colors duration-[var(--duration-fast)]",
          "cursor-pointer",
          hover
            ? "border-[var(--color-primary)] bg-[var(--color-surface-hover)]"
            : "border-[var(--color-border-strong)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)]",
        )}
      >
        {file ? (
          <>
            <FileText className="h-8 w-8 text-[var(--color-primary)]" />
            <p className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
              {file.name}
            </p>
            <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)] tabular-nums">
              {formatBytes(file.size)}
            </p>
          </>
        ) : (
          <>
            <UploadCloud className="h-8 w-8 text-[var(--color-text-tertiary)]" />
            <p className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {t("subscription:upload.drop_hint")}
            </p>
            <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("subscription:upload.max_size")}
            </p>
          </>
        )}
        <input
          ref={inputRef}
          type="file"
          className="hidden"
          accept=".yaml,.yml,.txt,application/yaml,application/x-yaml,text/yaml,text/plain"
          onChange={(e) => handleFiles(e.target.files)}
        />
      </div>

      {visibleError && (
        <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
          {visibleError}
        </p>
      )}
    </div>
  );
}
