import * as React from "react";
import { useTranslation } from "react-i18next";
import { Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useUpdateSettings, type SettingsMap } from "@/api/settings";

/**
 * Field descriptor used by SettingsForm. Each tab on the settings page passes
 * a small array — the form reads/writes the underlying SettingsMap through
 * these descriptors so we never bloat the component with type-specific code.
 */
export interface SettingsFieldDescriptor {
  /** The storage key consumed by /api/admin/settings — must match the backend literal. */
  key: string;
  /** Translation key (relative to the "settings" namespace) for the field label. */
  labelKey: string;
  /** Translation key for the hint text rendered below the input. Optional. */
  hintKey?: string;
  /** Input mode: "number" enforces inputmode=numeric for mobile UX. */
  inputMode?: "text" | "number";
  /** Validation hint values; numeric ranges are enforced before submit. */
  min?: number;
  max?: number;
}

interface SettingsFormProps {
  /** Field descriptors rendered in document order. */
  fields: SettingsFieldDescriptor[];
  /** Settings map fetched from /api/admin/settings (with sensitive keys masked). */
  initialValues: SettingsMap;
  /** Translation namespace prefix used to resolve labelKey / hintKey. Default "settings". */
  namespace?: string;
}

/**
 * Generic settings form rendered per-tab. The component is intentionally
 * minimal: every field is a string input + label + hint. Tabs requiring
 * richer controls (multi-select, color picker, etc.) should compose them
 * separately rather than overloading this component.
 *
 * The form tracks dirty state internally so "Save" stays disabled until the
 * admin has actually changed something. On submit it only sends the dirty
 * keys, which lets the backend keep masked secrets in place.
 */
export function SettingsForm({
  fields,
  initialValues,
  namespace = "settings",
}: SettingsFormProps) {
  const { t } = useTranslation([namespace, "common"]);
  const { handle: handleError } = useApiError();
  const update = useUpdateSettings();

  const [values, setValues] = React.useState<SettingsMap>(() => seed(fields, initialValues));
  const [errors, setErrors] = React.useState<Record<string, string>>({});

  // Re-seed when the upstream snapshot changes (after rotation, after another
  // tab persisted a change …).
  React.useEffect(() => {
    setValues(seed(fields, initialValues));
    setErrors({});
  }, [fields, initialValues]);

  const dirty = React.useMemo(() => {
    return fields.some(
      (f) => (values[f.key] ?? "") !== (initialValues[f.key] ?? ""),
    );
  }, [fields, values, initialValues]);

  const handleChange = (key: string, value: string) => {
    setValues((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload: SettingsMap = {};
    const nextErrors: Record<string, string> = {};
    for (const f of fields) {
      const current = values[f.key] ?? "";
      const baseline = initialValues[f.key] ?? "";
      if (current === baseline) continue;
      if (f.inputMode === "number") {
        const n = Number(current);
        if (!Number.isInteger(n)) {
          nextErrors[f.key] = t(`${namespace}:errors.invalid_number`);
          continue;
        }
        if (
          (f.min !== undefined && n < f.min) ||
          (f.max !== undefined && n > f.max)
        ) {
          nextErrors[f.key] = t(`${namespace}:errors.out_of_range`);
          continue;
        }
      }
      payload[f.key] = current;
    }
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    try {
      await update.mutateAsync(payload);
      toast.success(t(`${namespace}:actions.saved`));
    } catch (err) {
      handleError(err);
      toast.error(t(`${namespace}:actions.save_failed`));
    }
  };

  return (
    <form className="flex flex-col gap-4" onSubmit={handleSubmit} noValidate>
      {fields.map((field) => {
        const id = `setting-${field.key}`;
        const errMsg = errors[field.key];
        return (
          <div key={field.key} className="flex flex-col gap-2">
            <Label htmlFor={id}>{t(`${namespace}:${field.labelKey}`)}</Label>
            <Input
              id={id}
              type={field.inputMode === "number" ? "number" : "text"}
              inputMode={field.inputMode === "number" ? "numeric" : undefined}
              value={values[field.key] ?? ""}
              onChange={(e) => handleChange(field.key, e.target.value)}
              aria-invalid={!!errMsg}
              min={field.min}
              max={field.max}
            />
            {field.hintKey && !errMsg && (
              <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t(`${namespace}:${field.hintKey}`)}
              </p>
            )}
            {errMsg && (
              <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
                {errMsg}
              </p>
            )}
          </div>
        );
      })}
      <div className="flex justify-end">
        <Button type="submit" disabled={!dirty || update.isPending}>
          <Save className="mr-2 h-4 w-4" />
          {update.isPending
            ? t(`${namespace}:actions.saving`)
            : t(`${namespace}:actions.save`)}
        </Button>
      </div>
    </form>
  );
}

// seed initialises the form state from the GET response, filling in empty
// strings for keys the server hasn't persisted yet.
function seed(
  fields: SettingsFieldDescriptor[],
  initialValues: SettingsMap,
): SettingsMap {
  const out: SettingsMap = {};
  for (const f of fields) {
    out[f.key] = initialValues[f.key] ?? "";
  }
  return out;
}
