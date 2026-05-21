import * as React from "react";
import { useTranslation } from "react-i18next";
import { Field, Select } from "./_field";
import { Input } from "@/components/ui/input";
import type { OutputArgs, PipelineOperator } from "@/types/api";

interface OutputFormProps {
  operator: PipelineOperator;
  onChange: (next: PipelineOperator["params"]) => void;
}

/**
 * Output operator form.
 *
 *  - `format` is one of clash / clash_meta / raw (backend `validOutputFormats`).
 *  - `max_nodes` is an optional truncation cap; 0 / empty means "no cap".
 *    Number input is kept as a string in the UI and parsed at write-time so
 *    the user can clear the field cleanly.
 */
export function OutputForm({ operator, onChange }: OutputFormProps) {
  const { t } = useTranslation(["pipeline"]);
  const args = operator.params as OutputArgs;

  const update = (patch: Partial<OutputArgs>) => {
    const next: OutputArgs = {
      format: args.format ?? "clash",
      ...(args.max_nodes !== undefined ? { max_nodes: args.max_nodes } : {}),
      ...patch,
    };
    onChange(next);
  };

  const handleMaxNodes = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = e.target.value.trim();
    if (raw === "") {
      const { max_nodes: _drop, ...rest } = (args as OutputArgs) ?? {
        format: "clash",
      };
      void _drop;
      onChange({ format: rest.format ?? "clash" });
      return;
    }
    const parsed = Number(raw);
    if (!Number.isFinite(parsed) || parsed < 0) return;
    update({ max_nodes: Math.floor(parsed) });
  };

  return (
    <div className="flex flex-col gap-3">
      <Field
        label={t("pipeline:param_form.output.format_label")}
        htmlFor="output-format"
      >
        <Select
          id="output-format"
          data-testid="output-format"
          value={args.format || "clash"}
          onChange={(e) =>
            update({
              format: e.target.value as OutputArgs["format"],
            })
          }
        >
          <option value="clash">
            {t("pipeline:param_form.output.format_clash")}
          </option>
          <option value="clash_meta">
            {t("pipeline:param_form.output.format_clash_meta")}
          </option>
          <option value="raw">
            {t("pipeline:param_form.output.format_raw")}
          </option>
        </Select>
      </Field>

      <Field
        label={t("pipeline:param_form.output.max_nodes_label")}
        htmlFor="output-max-nodes"
        help={t("pipeline:param_form.output.max_nodes_help")}
      >
        <Input
          id="output-max-nodes"
          data-testid="output-max-nodes"
          type="number"
          inputMode="numeric"
          min={0}
          value={args.max_nodes ?? ""}
          onChange={handleMaxNodes}
          className="font-mono tabular-nums"
        />
      </Field>
    </div>
  );
}
