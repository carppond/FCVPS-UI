import * as React from "react";
import { useTranslation } from "react-i18next";
import { RotateCw, Search } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { cn } from "@/lib/cn";
import { useApiError } from "@/hooks/use-api-error";
import { useRotateShareTokenMutation } from "@/api/subscription";
import { ClientCard } from "./client-card";
import { CLIENT_CATALOG, type ClientPlatform } from "./client-catalog";

interface ShareUrlCardProps {
  subscriptionId: string;
  subscriptionName: string;
  /** Base URL `https://.../download/<name>?token=<share_token>` (no target). */
  shareUrl: string;
  /** False when share_token is missing — disables every card. */
  available: boolean;
}

type PlatformFilter = "all" | "desktop" | "mobile";

/**
 * Multi-client share-URL panel.
 *
 *  - Filter chips (all / desktop / mobile) + search input across the
 *    {@link CLIENT_CATALOG}.
 *  - Grid of ClientCard tiles, one per supported client (target query
 *    parameter is appended per card so each client receives the right
 *    producer format).
 *  - Token rotation control kept at the bottom (irreversible action).
 */
export function ShareUrlCard({
  subscriptionId,
  subscriptionName,
  shareUrl,
  available,
}: ShareUrlCardProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const rotate = useRotateShareTokenMutation();
  const [confirmOpen, setConfirmOpen] = React.useState(false);
  const [platform, setPlatform] = React.useState<PlatformFilter>("all");
  const [search, setSearch] = React.useState("");
  // Per-client short links live inside each ClientCard now (so each platform
  // gets a short URL pointing at its own target=... variant). Keep the
  // toolbar uncluttered.

  const filtered = React.useMemo(() => {
    const term = search.trim().toLowerCase();
    return CLIENT_CATALOG.filter((c) => {
      if (platform !== "all") {
        if (c.platform !== platform && c.platform !== "both") return false;
      }
      if (!term) return true;
      return (
        c.name.toLowerCase().includes(term) ||
        c.target.toLowerCase().includes(term)
      );
    });
  }, [platform, search]);

  const confirmRotate = async () => {
    try {
      await rotate.mutateAsync(subscriptionId);
      toast.success(t("subscription:detail.share.rotate_success"));
      setConfirmOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
        <header className="flex flex-col gap-1">
          <h3 className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
            {t("subscription:detail.share.title")}
          </h3>
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("subscription:detail.share.url_hint")}
          </p>
        </header>

        <div className="mt-4 flex flex-wrap items-center gap-2">
          <PlatformChips value={platform} onChange={setPlatform} />
          <div className="relative ml-auto flex-1 min-w-[200px] max-w-xs">
            <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("subscription:detail.share.search_placeholder")}
              className="pl-7 text-[var(--font-size-xs)]"
            />
          </div>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((client) => (
            <ClientCard
              key={client.id}
              client={client}
              baseUrl={shareUrl}
              subscriptionName={subscriptionName}
              disabled={!available}
            />
          ))}
        </div>

        {filtered.length === 0 && (
          <p className="mt-4 text-center text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            —
          </p>
        )}
      </div>

      <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
        <h3 className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {t("subscription:detail.share.rotate_title")}
        </h3>
        <p className="mt-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("subscription:detail.share.rotate_confirm")}
        </p>
        <Button
          variant="outline"
          className="mt-3"
          onClick={() => setConfirmOpen(true)}
          disabled={rotate.isPending}
        >
          <RotateCw className="mr-2 h-4 w-4" />
          {t("subscription:actions.rotate_token")}
        </Button>
      </div>

      <Dialog
        open={confirmOpen}
        onOpenChange={(o) => !o && !rotate.isPending && setConfirmOpen(false)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.share.rotate_title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:detail.share.rotate_confirm")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmOpen(false)}
              disabled={rotate.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmRotate}
              disabled={rotate.isPending}
            >
              {t("subscription:actions.rotate_token")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function PlatformChips({
  value,
  onChange,
}: {
  value: PlatformFilter;
  onChange: (v: PlatformFilter) => void;
}) {
  const { t } = useTranslation("subscription");
  const options: PlatformFilter[] = ["all", "desktop", "mobile"];
  return (
    <div
      className={cn(
        "inline-flex rounded-[var(--radius-md)] border border-[var(--color-border)] p-0.5",
        "bg-[var(--color-bg-elevated)]",
      )}
      role="tablist"
    >
      {options.map((opt) => (
        <button
          key={opt}
          role="tab"
          aria-selected={value === opt}
          onClick={() => onChange(opt)}
          className={cn(
            "rounded-[var(--radius-sm)] px-3 py-1 text-[var(--font-size-xs)] transition-colors",
            value === opt
              ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)] font-semibold shadow-[var(--shadow-sm)]"
              : "text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)]",
          )}
        >
          {t(`detail.share.platforms.${opt as ClientPlatform | "all"}`)}
        </button>
      ))}
    </div>
  );
}
