import { useTranslation } from "react-i18next";
import { Field } from "./_field";
import { ChipList } from "./_chip-list";
import type { DedupeArgs, PipelineOperator } from "@/types/api";

interface DedupeFormProps {
  operator: PipelineOperator;
  onChange: (next: PipelineOperator["params"]) => void;
}

export function DedupeForm({ operator, onChange }: DedupeFormProps) {
  const { t } = useTranslation(["pipeline"]);
  const args = operator.params as DedupeArgs;
  const fields = Array.isArray(args.fields) ? args.fields : [];

  return (
    <div className="flex flex-col gap-3">
      <Field
        label={t("pipeline:param_form.dedupe.fields_label")}
        htmlFor="dedupe-fields"
        help={t("pipeline:param_form.dedupe.fields_help")}
      >
        <ChipList
          id="dedupe-fields"
          value={fields}
          onChange={(next) => onChange({ fields: next } satisfies DedupeArgs)}
          placeholder={t("pipeline:param_form.dedupe.fields_placeholder")}
        />
      </Field>
    </div>
  );
}
