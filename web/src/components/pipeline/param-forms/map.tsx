import { useTranslation } from "react-i18next";
import { Field, Select } from "./_field";
import { Input } from "@/components/ui/input";
import type { MapArgs, PipelineOperator } from "@/types/api";

/** Writable fields, kept in sync with backend `allowedMapFields`. */
const MAP_FIELDS = [
  "name",
  "server",
  "port",
  "tag",
  "network",
  "sni",
  "host",
  "path",
  "password",
] as const;

interface MapFormProps {
  operator: PipelineOperator;
  onChange: (next: PipelineOperator["params"]) => void;
}

export function MapForm({ operator, onChange }: MapFormProps) {
  const { t } = useTranslation(["pipeline"]);
  const args = operator.params as MapArgs;

  const update = (patch: Partial<MapArgs>) => {
    const next: MapArgs = {
      field: args.field ?? "tag",
      value: args.value ?? "",
      ...patch,
    };
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-3">
      <Field
        label={t("pipeline:param_form.map.field_label")}
        htmlFor="map-field"
        help={t("pipeline:param_form.map.field_help")}
      >
        <Select
          id="map-field"
          data-testid="map-field"
          value={args.field || "tag"}
          onChange={(e) => update({ field: e.target.value })}
        >
          {MAP_FIELDS.map((f) => (
            <option key={f} value={f}>
              {f}
            </option>
          ))}
        </Select>
      </Field>

      <Field
        label={t("pipeline:param_form.map.value_label")}
        htmlFor="map-value"
        help={t("pipeline:param_form.map.value_help")}
      >
        <Input
          id="map-value"
          data-testid="map-value"
          value={args.value ?? ""}
          onChange={(e) => update({ value: e.target.value })}
          placeholder={t("pipeline:param_form.map.value_placeholder")}
          spellCheck={false}
          autoComplete="off"
          className="font-mono"
        />
      </Field>
    </div>
  );
}
