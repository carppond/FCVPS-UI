import * as React from "react";
import { useTranslation } from "react-i18next";
import { Field, Select } from "./_field";
import { Input } from "@/components/ui/input";
import type { PipelineOperator, RegexRenameArgs } from "@/types/api";

/** v1: backend allows only `name` / `tag` as targets. */
const RENAME_FIELDS = ["name", "tag"] as const;

interface RegexRenameFormProps {
  operator: PipelineOperator;
  onChange: (next: PipelineOperator["params"]) => void;
}

/**
 * Regex-rename form with live regex validation. We compile via `new RegExp`
 * client-side so users get instant feedback before Save round-trips to the
 * Go backend (which uses RE2 — most patterns valid in both).
 */
export function RegexRenameForm({
  operator,
  onChange,
}: RegexRenameFormProps) {
  const { t } = useTranslation(["pipeline"]);
  const args = operator.params as RegexRenameArgs & { field?: string };

  const regexError = React.useMemo(() => {
    if (!args.pattern) return null;
    try {
      new RegExp(args.pattern);
      return null;
    } catch (err) {
      return err instanceof Error ? err.message : String(err);
    }
  }, [args.pattern]);

  const update = (patch: Partial<RegexRenameArgs & { field?: string }>) => {
    const next: RegexRenameArgs & { field?: string } = {
      pattern: args.pattern ?? "",
      replacement: args.replacement ?? "",
      ...(args.field ? { field: args.field } : {}),
      ...patch,
    };
    onChange(next as PipelineOperator["params"]);
  };

  return (
    <div className="flex flex-col gap-3">
      <Field
        label={t("pipeline:param_form.regex_rename.pattern_label")}
        htmlFor="rename-pattern"
        error={
          regexError
            ? t("pipeline:param_form.regex_rename.regex_invalid", {
                message: regexError,
              })
            : undefined
        }
        help={
          !regexError && args.pattern
            ? t("pipeline:param_form.regex_rename.regex_valid")
            : undefined
        }
      >
        <Input
          id="rename-pattern"
          data-testid="rename-pattern"
          value={args.pattern ?? ""}
          onChange={(e) => update({ pattern: e.target.value })}
          placeholder={t(
            "pipeline:param_form.regex_rename.pattern_placeholder",
          )}
          spellCheck={false}
          autoComplete="off"
          className="font-mono"
        />
      </Field>

      <Field
        label={t("pipeline:param_form.regex_rename.replacement_label")}
        htmlFor="rename-replacement"
      >
        <Input
          id="rename-replacement"
          data-testid="rename-replacement"
          value={args.replacement ?? ""}
          onChange={(e) => update({ replacement: e.target.value })}
          placeholder={t(
            "pipeline:param_form.regex_rename.replacement_placeholder",
          )}
          spellCheck={false}
          autoComplete="off"
          className="font-mono"
        />
      </Field>

      <Field
        label={t("pipeline:param_form.regex_rename.field_label")}
        htmlFor="rename-field"
      >
        <Select
          id="rename-field"
          data-testid="rename-field"
          value={args.field || "name"}
          onChange={(e) => update({ field: e.target.value })}
        >
          {RENAME_FIELDS.map((f) => (
            <option key={f} value={f}>
              {f}
            </option>
          ))}
        </Select>
      </Field>
    </div>
  );
}
