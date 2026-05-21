import {
  ArrowDownUp,
  Filter as FilterIcon,
  Layers,
  Replace,
  SquareDashed,
  Wand2,
  type LucideIcon,
} from "lucide-react";
import type { OperatorType } from "@/types/api";

/**
 * Static metadata for the 6 operator types: icon + i18n keys (name / description).
 *
 *  - `id` is the on-the-wire `OperatorType` enum used by the backend.
 *  - `iconComponent` is a lucide-react component — instantiated in the
 *    consumer so it can pass `className` / size props.
 *  - `nameKey` / `descriptionKey` resolve via the `pipeline` namespace.
 */
export interface OperatorMeta {
  id: OperatorType;
  iconComponent: LucideIcon;
  nameKey: string;
  descriptionKey: string;
}

export const OPERATOR_META: readonly OperatorMeta[] = [
  {
    id: "filter",
    iconComponent: FilterIcon,
    nameKey: "pipeline:operators.filter.name",
    descriptionKey: "pipeline:operators.filter.description",
  },
  {
    id: "map",
    iconComponent: Wand2,
    nameKey: "pipeline:operators.map.name",
    descriptionKey: "pipeline:operators.map.description",
  },
  {
    id: "sort",
    iconComponent: ArrowDownUp,
    nameKey: "pipeline:operators.sort.name",
    descriptionKey: "pipeline:operators.sort.description",
  },
  {
    id: "dedupe",
    iconComponent: SquareDashed,
    nameKey: "pipeline:operators.dedupe.name",
    descriptionKey: "pipeline:operators.dedupe.description",
  },
  {
    id: "regex_rename",
    iconComponent: Replace,
    nameKey: "pipeline:operators.regex_rename.name",
    descriptionKey: "pipeline:operators.regex_rename.description",
  },
  {
    id: "output",
    iconComponent: Layers,
    nameKey: "pipeline:operators.output.name",
    descriptionKey: "pipeline:operators.output.description",
  },
] as const;

const META_BY_ID: Readonly<Record<OperatorType, OperatorMeta>> = Object.freeze(
  OPERATOR_META.reduce(
    (acc, meta) => {
      acc[meta.id] = meta;
      return acc;
    },
    {} as Record<OperatorType, OperatorMeta>,
  ),
);

export function getOperatorMeta(type: OperatorType): OperatorMeta {
  return META_BY_ID[type];
}
