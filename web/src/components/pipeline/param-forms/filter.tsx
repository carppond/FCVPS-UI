import * as React from "react";
import { useTranslation } from "react-i18next";
import { Field } from "./_field";
import { Input } from "@/components/ui/input";
import type { FilterArgs, PipelineOperator } from "@/types/api";

interface FilterFormProps {
  operator: PipelineOperator;
  onChange: (next: PipelineOperator["params"]) => void;
}

/**
 * Filter operator form — exposes the single `expr` field as a text input.
 * Backend (`internal/pipeline/op_filter.go`) treats empty expr as "keep all",
 * so we don't enforce a required validation here.
 */
export function FilterForm({ operator, onChange }: FilterFormProps) {
  const { t } = useTranslation(["pipeline"]);
  const args = operator.params as FilterArgs;

  const handleExprChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const next: FilterArgs = { expr: e.target.value };
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-3">
      <Field
        label={t("pipeline:param_form.filter.expr_label")}
        htmlFor="filter-expr"
        help={t("pipeline:param_form.filter.expr_help")}
      >
        <Input
          id="filter-expr"
          data-testid="filter-expr"
          value={args.expr ?? ""}
          onChange={handleExprChange}
          placeholder={t("pipeline:param_form.filter.expr_placeholder")}
          spellCheck={false}
          autoComplete="off"
          className="font-mono"
        />
      </Field>
    </div>
  );
}
