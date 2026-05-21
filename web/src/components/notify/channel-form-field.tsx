import * as React from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { ChannelKindField } from "@/api/notify";
import type { ChannelKind } from "@/types/api";

/**
 * Schema-driven field row used by ChannelForm. Extracted into its own module
 * to keep the parent form under the size budget (coding standards §1).
 *
 * The renderer keeps the `unknown`-cast surface narrow — every union arm
 * explicitly coerces to the shape the matching `<input>` expects.
 */
export function FieldRow({
  kind,
  field,
  value,
  error,
  onChange,
}: {
  kind: ChannelKind;
  field: ChannelKindField;
  value: unknown;
  error?: string;
  onChange: (v: unknown) => void;
}) {
  const { t } = useTranslation("notify");
  const labelKey = `notify:kinds.${kind}.fields.${field.name}.label`;
  const helpKey = `notify:kinds.${kind}.fields.${field.name}.help`;

  const label = t(labelKey, { defaultValue: field.label ?? field.name });
  const help = t(helpKey, { defaultValue: field.help ?? "" });

  return (
    <div className="flex flex-col gap-[var(--spacing-1)]">
      <Label htmlFor={`field-${field.name}`}>
        {label}
        {field.required && (
          <span className="ml-1 text-[var(--color-error)]">*</span>
        )}
      </Label>
      {renderInput(field, value, onChange)}
      {help && (
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {help}
        </p>
      )}
      {error && (
        <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
          {error}
        </p>
      )}
    </div>
  );
}

function renderInput(
  field: ChannelKindField,
  value: unknown,
  onChange: (v: unknown) => void,
): React.ReactNode {
  const id = `field-${field.name}`;
  const placeholder = field.placeholder ?? "";

  switch (field.type) {
    case "password":
      return (
        <Input
          id={id}
          type="password"
          autoComplete="new-password"
          value={typeof value === "string" ? value : ""}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          data-testid={`notify-field-${field.name}`}
        />
      );

    case "number":
      return (
        <Input
          id={id}
          type="number"
          inputMode="numeric"
          value={
            typeof value === "number"
              ? value
              : typeof value === "string"
                ? value
                : ""
          }
          placeholder={placeholder}
          onChange={(e) => {
            const n = e.target.value === "" ? "" : Number(e.target.value);
            onChange(typeof n === "number" && Number.isNaN(n) ? "" : n);
          }}
          data-testid={`notify-field-${field.name}`}
        />
      );

    case "boolean":
      return (
        <label className="inline-flex items-center gap-[var(--spacing-2)] text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
          <input
            id={id}
            type="checkbox"
            checked={Boolean(value)}
            onChange={(e) => onChange(e.target.checked)}
            className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
            data-testid={`notify-field-${field.name}`}
          />
          {placeholder || ""}
        </label>
      );

    case "string[]":
      return (
        <Input
          id={id}
          value={Array.isArray(value) ? value.join(", ") : ""}
          placeholder={placeholder}
          onChange={(e) =>
            onChange(
              e.target.value
                .split(/[,\n]/)
                .map((s) => s.trim())
                .filter(Boolean),
            )
          }
          data-testid={`notify-field-${field.name}`}
        />
      );

    case "select":
      return (
        <select
          id={id}
          value={typeof value === "string" ? value : ""}
          onChange={(e) => onChange(e.target.value)}
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          data-testid={`notify-field-${field.name}`}
        >
          <option value="">--</option>
          {(field.options ?? []).map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label ?? opt.value}
            </option>
          ))}
        </select>
      );

    case "map":
      // Render maps as one entry per line — `key: value`. This is good
      // enough for headers; the parsed value goes back as Record<string,string>.
      return (
        <textarea
          id={id}
          value={mapToText(value)}
          placeholder={placeholder || "Content-Type: application/json"}
          rows={3}
          onChange={(e) => onChange(textToMap(e.target.value))}
          className="rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          data-testid={`notify-field-${field.name}`}
        />
      );

    case "string":
    default:
      return (
        <Input
          id={id}
          type="text"
          value={typeof value === "string" ? value : ""}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          data-testid={`notify-field-${field.name}`}
        />
      );
  }
}

function mapToText(v: unknown): string {
  if (!v || typeof v !== "object") return "";
  return Object.entries(v as Record<string, string>)
    .map(([k, val]) => `${k}: ${val}`)
    .join("\n");
}

function textToMap(text: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of text.split(/\n/)) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const idx = trimmed.indexOf(":");
    if (idx < 0) continue;
    const key = trimmed.slice(0, idx).trim();
    const val = trimmed.slice(idx + 1).trim();
    if (key) result[key] = val;
  }
  return result;
}
