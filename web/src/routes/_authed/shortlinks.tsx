import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { ShortLinkList } from "@/components/shortlink/shortlink-list";
import { useCreateShortLink } from "@/api/shortlink";
import i18next from "@/lib/i18n";
import shortlinkZhCN from "@/locales/zh-CN/shortlink.json";
import shortlinkEn from "@/locales/en/shortlink.json";
import shortlinkJa from "@/locales/ja/shortlink.json";
import shortlinkKo from "@/locales/ko/shortlink.json";

// Register the locale bundle once when the route module loads. Lazy ns
// loading is the project default; doing it inside the route module keeps
// the first-screen bundle small.
i18next.addResourceBundle("zh-CN", "shortlink", shortlinkZhCN);
i18next.addResourceBundle("en", "shortlink", shortlinkEn);
i18next.addResourceBundle("ja", "shortlink", shortlinkJa);
i18next.addResourceBundle("ko", "shortlink", shortlinkKo);

export const Route = createFileRoute("/_authed/shortlinks")({
  component: ShortLinksPage,
});

function ShortLinksPage() {
  const { t } = useTranslation(["shortlink", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateShortLink();

  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [targetUrl, setTargetUrl] = React.useState("");
  const [expiresAt, setExpiresAt] = React.useState("");

  const openCreate = () => {
    setTargetUrl("");
    setExpiresAt("");
    setDialogOpen(true);
  };

  const submit = async () => {
    if (!targetUrl.trim()) {
      toast.error(t("shortlink:create.error_required"));
      return;
    }
    try {
      const expiresMs = expiresAt ? new Date(expiresAt).getTime() : 0;
      await createMutation.mutateAsync({
        target_url: targetUrl.trim(),
        expires_at: expiresMs,
      });
      toast.success(t("shortlink:create.success"));
      setDialogOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
            {t("shortlink:page.title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("shortlink:page.description")}
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          {t("shortlink:page.create")}
        </Button>
      </header>

      <ShortLinkList onCreate={openCreate} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("shortlink:create.title")}</DialogTitle>
            <DialogDescription>
              {t("shortlink:create.description")}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div>
              <Label htmlFor="target-url">{t("shortlink:create.target_label")}</Label>
              <Input
                id="target-url"
                value={targetUrl}
                onChange={(e) => setTargetUrl(e.target.value)}
                placeholder={t("shortlink:create.target_placeholder")}
              />
            </div>
            <div>
              <Label htmlFor="expires-at">{t("shortlink:create.expires_label")}</Label>
              <Input
                id="expires-at"
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
              <p className="mt-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("shortlink:create.expires_hint")}
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDialogOpen(false)}
              disabled={createMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button onClick={submit} disabled={createMutation.isPending}>
              {t("common:actions.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
