import { useTranslation } from "react-i18next";
import { Field, Select } from "./_field";
import type { PipelineOperator, SortArgs } from "@/types/api";

/** Recognised by backend `validSortKeys`. */
const SORT_KEYS = [
  "name",
  "server",
  "port",
  "tag",
  "protocol",
  "latency",
] as const;

interface SortFormProps {
  operator: PipelineOperator;
  onChange: (next: PipelineOperator["params"]) => void;
}

export function SortForm({ operator, onChange }: SortFormProps) {
  const { t } = useTranslation(["pipeline"]);
  const args = operator.params as SortArgs;

  const update = (patch: Partial<SortArgs>) => {
    const next: SortArgs = {
      key: args.key ?? "name",
      order: args.order ?? "asc",
      ...patch,
    };
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-3">
      <Field
        label={t("pipeline:param_form.sort.key_label")}
        htmlFor="sort-key"
      >
        <Select
          id="sort-key"
          data-testid="sort-key"
          value={args.key || "name"}
          onChange={(e) => update({ key: e.target.value })}
        >
          {SORT_KEYS.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </Select>
      </Field>

      <Field
        label={t("pipeline:param_form.sort.order_label")}
        htmlFor="sort-order"
      >
        <Select
          id="sort-order"
          data-testid="sort-order"
          value={args.order || "asc"}
          onChange={(e) =>
            update({ order: e.target.value === "desc" ? "desc" : "asc" })
          }
        >
          <option value="asc">
            {t("pipeline:param_form.sort.order_asc")}
          </option>
          <option value="desc">
            {t("pipeline:param_form.sort.order_desc")}
          </option>
        </Select>
      </Field>
    </div>
  );
}
