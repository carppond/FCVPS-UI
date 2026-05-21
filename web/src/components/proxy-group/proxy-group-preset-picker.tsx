import * as React from "react";
import { useTranslation } from "react-i18next";
import { Check } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useCreateProxyGroup,
  useProxyGroupPresets,
} from "@/api/proxy-group";
import type { ProxyGroupPreset } from "@/types/api";

interface ProxyGroupPresetPickerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type PresetTab = "core" | "region" | "app";

/**
 * Backend presets ship with structured ids like "node-select" / "region-hk".
 * We bucket them into tabs by prefix so the dialog mirrors the visual grouping
 * specified in the PRD even though the API itself doesn't carry an explicit
 * category field on proxy groups.
 */
function categoryFor(id: string): PresetTab {
  if (id.startsWith("region-")) return "region";
  if (id.startsWith("app-")) return "app";
  return "core";
}

export function ProxyGroupPresetPicker({
  open,
  onOpenChange,
}: ProxyGroupPresetPickerProps) {
  const { t } = useTranslation(["proxy-group", "common"]);
  const { handle: handleError } = useApiError();
  const { data, isLoading, isError, error, refetch } = useProxyGroupPresets();
  const createMutation = useCreateProxyGroup();

  const [tab, setTab] = React.useState<PresetTab>("core");
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [submitting, setSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setSelected(new Set());
      setTab("core");
    }
  }, [open]);

  const byCategory = React.useMemo(() => {
    const acc: Record<PresetTab, ProxyGroupPreset[]> = {
      core: [],
      region: [],
      app: [],
    };
    for (const p of data ?? []) {
      acc[categoryFor(p.id)].push(p);
    }
    return acc;
  }, [data]);

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const onAdd = async () => {
    if (!data || selected.size === 0) return;
    setSubmitting(true);
    let ok = 0;
    let failed = 0;
    try {
      const ordered = data.filter((p) => selected.has(p.id));
      let sortBase = Date.now() % 100000;
      for (const preset of ordered) {
        try {
          await createMutation.mutateAsync({
            name: preset.name,
            type: preset.type,
            icon: preset.icon,
            test_url: preset.test_url,
            test_interval: preset.test_interval,
            filter: preset.filter,
            include_all: preset.include_all,
            member_proxies: preset.member_proxies,
            member_groups: preset.member_groups,
            sort_order: sortBase++,
          });
          ok += 1;
        } catch {
          failed += 1;
        }
      }
      if (failed === 0) {
        toast.success(t("proxy-group:presets.added_toast", { count: ok }));
      } else {
        toast.warning(
          t("proxy-group:presets.added_with_failures", { ok, failed }),
        );
      }
      onOpenChange(false);
    } catch (err) {
      handleError(err);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !submitting && onOpenChange(o)}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t("proxy-group:presets.dialog_title")}</DialogTitle>
          <DialogDescription>
            {t("proxy-group:presets.dialog_description")}
          </DialogDescription>
        </DialogHeader>

        {isError && (
          <ErrorState
            message={error?.message ?? t("proxy-group:error.presets_failed")}
            onRetry={() => void refetch()}
            retryLabel={t("common:actions.retry")}
          />
        )}

        {!isError && (
          <Tabs value={tab} onValueChange={(v) => setTab(v as PresetTab)}>
            <TabsList className="w-full">
              <TabsTrigger value="core" className="flex-1">
                {t("proxy-group:presets.tabs.core")}
              </TabsTrigger>
              <TabsTrigger value="region" className="flex-1">
                {t("proxy-group:presets.tabs.region")}
              </TabsTrigger>
              <TabsTrigger value="app" className="flex-1">
                {t("proxy-group:presets.tabs.app")}
              </TabsTrigger>
            </TabsList>

            {(["core", "region", "app"] as const).map((key) => (
              <TabsContent
                key={key}
                value={key}
                className="max-h-[60vh] overflow-y-auto"
              >
                {isLoading ? (
                  <PresetGridSkeleton />
                ) : (
                  <PresetGrid
                    presets={byCategory[key]}
                    selected={selected}
                    onToggle={toggle}
                  />
                )}
              </TabsContent>
            ))}
          </Tabs>
        )}

        <DialogFooter className="mt-2">
          <div className="mr-auto flex items-center text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("proxy-group:presets.select_count", { count: selected.size })}
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t("proxy-group:form.cancel")}
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => void onAdd()}
            disabled={submitting || selected.size === 0}
          >
            {t("proxy-group:presets.add_selected", { count: selected.size })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

interface PresetGridProps {
  presets: ProxyGroupPreset[];
  selected: Set<string>;
  onToggle: (id: string) => void;
}

function PresetGrid({ presets, selected, onToggle }: PresetGridProps) {
  if (presets.length === 0) {
    return (
      <p className="py-8 text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        —
      </p>
    );
  }
  return (
    <ul className="grid grid-cols-1 gap-2 py-2 sm:grid-cols-2">
      {presets.map((p) => {
        const isSel = selected.has(p.id);
        return (
          <li key={p.id}>
            <button
              type="button"
              role="checkbox"
              aria-checked={isSel}
              data-testid={`proxy-group-preset-${p.id}`}
              onClick={() => onToggle(p.id)}
              className={cn(
                "flex w-full items-start gap-3 rounded-[var(--radius-lg)] border p-3 text-left",
                "transition-colors duration-[var(--duration-fast)]",
                isSel
                  ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10"
                  : "border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)]",
              )}
            >
              <span
                className={cn(
                  "mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-md)]",
                  isSel
                    ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)]"
                    : "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
                )}
                aria-hidden
              >
                {isSel ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <span className="text-base">{p.icon ?? "·"}</span>
                )}
              </span>
              <span className="flex flex-1 flex-col gap-1">
                <span className="flex items-center gap-2">
                  <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                    {p.name}
                  </span>
                  <Badge variant="outline">{p.type}</Badge>
                </span>
                {p.description && (
                  <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                    {p.description}
                  </span>
                )}
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function PresetGridSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-2 py-2 sm:grid-cols-2" aria-hidden>
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="flex items-start gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] p-3"
        >
          <Skeleton className="h-8 w-8 rounded-[var(--radius-md)]" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-56" />
          </div>
        </div>
      ))}
    </div>
  );
}
