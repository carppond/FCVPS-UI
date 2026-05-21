import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useSetTrafficLimitMutation,
  useSetTrafficThresholdMutation,
} from "@/api/traffic";

interface ThresholdConfigProps {
  /** Caller's role. Only admins can save; non-admins see a read-only state. */
  isAdmin: boolean;
  /** Current monthly limit in BYTES. 0 means unlimited. */
  currentLimit: number;
  /** Current configured percentages (defaults to [80, 90, 100]). */
  currentLevels?: number[];
  className?: string;
}

const PRESET_LEVELS = [80, 90, 100] as const;

/**
 * Admin-facing card that lets the operator toggle the 80 / 90 / 100% alert
 * levels and edit the monthly limit (entered in GB and persisted as bytes
 * so the backend formula stays in canonical units). Non-admins still see the
 * card but it is rendered in read-only mode.
 */
export function ThresholdConfig({
  isAdmin,
  currentLimit,
  currentLevels,
  className,
}: ThresholdConfigProps) {
  const { t } = useTranslation(["traffic", "common"]);
  const { handle } = useApiError();
  const setThresholdMutation = useSetTrafficThresholdMutation();
  const setLimitMutation = useSetTrafficLimitMutation();

  const [levels, setLevels] = useState<number[]>(
    currentLevels && currentLevels.length > 0
      ? Array.from(new Set(currentLevels))
      : [80, 90, 100],
  );
  const [limitGB, setLimitGB] = useState<string>(
    currentLimit > 0 ? bytesToGB(currentLimit).toString() : "",
  );

  useEffect(() => {
    setLimitGB(currentLimit > 0 ? bytesToGB(currentLimit).toString() : "");
  }, [currentLimit]);

  const toggleLevel = (level: number) => {
    if (!isAdmin) return;
    setLevels((prev) =>
      prev.includes(level) ? prev.filter((l) => l !== level) : [...prev, level].sort((a, b) => a - b),
    );
  };

  const onSaveThreshold = async () => {
    if (!isAdmin) return;
    try {
      await setThresholdMutation.mutateAsync({ percents: levels });
      toast.success(t("traffic:threshold.save_success"));
    } catch (err) {
      handle(err);
    }
  };

  const onSaveLimit = async () => {
    if (!isAdmin) return;
    const n = Number(limitGB);
    if (Number.isNaN(n) || n < 0) {
      toast.error(t("traffic:limit.save_failed"));
      return;
    }
    try {
      await setLimitMutation.mutateAsync({ total_limit: gbToBytes(n) });
      toast.success(t("traffic:limit.save_success"));
    } catch (err) {
      handle(err);
    }
  };

  return (
    <div
      className={cn(
        "flex flex-col gap-[var(--spacing-6)] rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]",
        className,
      )}
    >
      {/* Threshold percent toggle row */}
      <div className="flex flex-col gap-[var(--spacing-3)]">
        <div>
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("traffic:threshold.title")}
          </h3>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("traffic:threshold.subtitle")}
          </p>
        </div>
        <div className="flex flex-wrap gap-[var(--spacing-2)]">
          {PRESET_LEVELS.map((lvl) => {
            const active = levels.includes(lvl);
            return (
              <button
                key={lvl}
                type="button"
                disabled={!isAdmin}
                onClick={() => toggleLevel(lvl)}
                className={cn(
                  "inline-flex items-center gap-[var(--spacing-2)] rounded-[var(--radius-full)] border px-[var(--spacing-4)] py-[var(--spacing-2)] text-[var(--font-size-sm)] font-medium transition-colors duration-[var(--duration-fast)]",
                  active
                    ? "border-transparent bg-[var(--color-primary)] text-[var(--color-primary-foreground)]"
                    : "border-[var(--color-border-strong)] bg-transparent text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-hover)]",
                  !isAdmin && "cursor-not-allowed opacity-60",
                )}
              >
                <span
                  className={cn(
                    "h-2 w-2 rounded-[var(--radius-full)]",
                    active
                      ? "bg-[var(--color-primary-foreground)]"
                      : "bg-[var(--color-text-disabled)]",
                  )}
                />
                {t(`traffic:threshold.level_${lvl}`)}
              </button>
            );
          })}
        </div>
        {isAdmin ? (
          <div>
            <Button
              type="button"
              size="sm"
              onClick={onSaveThreshold}
              disabled={setThresholdMutation.isPending}
            >
              {t("common:actions.save")}
            </Button>
          </div>
        ) : (
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("traffic:threshold.admin_only")}
          </p>
        )}
      </div>

      {/* Limit input row */}
      <div className="flex flex-col gap-[var(--spacing-3)]">
        <div>
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("traffic:limit.title")}
          </h3>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("traffic:limit.subtitle")}
          </p>
        </div>
        <div className="flex flex-wrap items-end gap-[var(--spacing-3)]">
          <div className="flex flex-col gap-[var(--spacing-1)]">
            <Label htmlFor="traffic-limit-input">
              {t("traffic:limit.input_label")}
            </Label>
            <Input
              id="traffic-limit-input"
              type="number"
              min="0"
              step="0.1"
              disabled={!isAdmin}
              value={limitGB}
              onChange={(e) => setLimitGB(e.target.value)}
              className="w-40"
            />
          </div>
          {isAdmin ? (
            <Button
              type="button"
              size="sm"
              onClick={onSaveLimit}
              disabled={setLimitMutation.isPending}
            >
              {t("common:actions.save")}
            </Button>
          ) : null}
        </div>
        {!isAdmin ? (
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("traffic:limit.admin_only")}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function gbToBytes(gb: number): number {
  return Math.round(gb * 1024 * 1024 * 1024);
}

function bytesToGB(bytes: number): number {
  return Number((bytes / 1024 / 1024 / 1024).toFixed(3));
}
