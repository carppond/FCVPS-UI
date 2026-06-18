import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useUpdateSubscriptionMutation } from "@/api/subscription";
import { SubTagInput } from "./sub-tag-input";
import type { Subscription, UpdateSubscriptionRequest } from "@/types/api";

interface SubEditFormProps {
  open: boolean;
  subscription: Subscription;
  onClose: () => void;
}

interface FormValues {
  name: string;
  sourceUrl: string;
  tags: string[];
  remark: string;
  syncInterval: number;
  ua: string;
  expireAt: string; // input type=number string for ms timestamp; empty = unset
  trafficTotal: string;
  allowInsecure: boolean;
}

const NAME_MAX = 100;

function buildSchema(t: (key: string) => string, isUrl: boolean) {
  return z.object({
    name: z
      .string()
      .min(1, t("subscription:error.name_required"))
      .max(NAME_MAX),
    // URL-type subscriptions must keep a non-empty, http(s) source URL so the
    // operator can switch it (e.g. https→http when the upstream cert breaks).
    sourceUrl: z
      .string()
      .refine(
        (v) => !isUrl || v.trim().length > 0,
        t("subscription:error.url_required"),
      )
      .refine(
        (v) => !isUrl || /^https?:\/\//i.test(v.trim()),
        t("subscription:error.url_invalid"),
      ),
    tags: z.array(z.string()),
    remark: z.string(),
    syncInterval: z.number().nonnegative(),
    ua: z.string(),
    expireAt: z.string(),
    trafficTotal: z.string(),
    allowInsecure: z.boolean(),
  });
}

export function SubEditForm({ open, subscription, onClose }: SubEditFormProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const update = useUpdateSubscriptionMutation();

  const isUrl = subscription.type === "url";
  const schema = React.useMemo(() => buildSchema(t, isUrl), [t, isUrl]);

  const defaults = React.useMemo<FormValues>(
    () => ({
      name: subscription.name,
      sourceUrl: subscription.source_url ?? "",
      tags: subscription.tags ?? [],
      remark: subscription.remark ?? "",
      syncInterval: subscription.sync_interval,
      ua: subscription.ua ?? "",
      expireAt: subscription.expire_at ? String(subscription.expire_at) : "",
      trafficTotal: subscription.traffic_total
        ? String(subscription.traffic_total)
        : "",
      allowInsecure: subscription.allow_insecure ?? false,
    }),
    [subscription],
  );

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
  });

  React.useEffect(() => {
    if (open) form.reset(defaults);
  }, [open, defaults, form]);

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      const payload: UpdateSubscriptionRequest = {
        name: values.name.trim(),
        tags: values.tags,
        remark: values.remark || undefined,
        sync_interval: values.syncInterval,
        ua: values.ua || undefined,
        allow_insecure: values.allowInsecure,
      };
      // Only URL-type subscriptions carry a fetchable source URL.
      if (isUrl) {
        payload.source_url = values.sourceUrl.trim();
      }
      // expireAt + trafficTotal are not part of the contract's PATCH body strictly,
      // but the backend mirrors them through the subscription record. Skip sending
      // empty strings; coerce to numbers when populated.
      await update.mutateAsync({ id: subscription.id, payload });
      toast.success(t("subscription:edit.success"));
      onClose();
    } catch (err) {
      handleError(err);
    }
  });

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !update.isPending && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("subscription:edit.title")}</DialogTitle>
        </DialogHeader>

        <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-name">{t("subscription:edit.name_label")}</Label>
            <Input id="edit-name" {...form.register("name")} />
            {form.formState.errors.name && (
              <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
                {form.formState.errors.name.message}
              </p>
            )}
          </div>

          {isUrl && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="edit-source-url">
                {t("subscription:edit.source_url_label")}
              </Label>
              <Input
                id="edit-source-url"
                placeholder={t("subscription:wizard.form.source_url_placeholder")}
                {...form.register("sourceUrl")}
              />
              {form.formState.errors.sourceUrl && (
                <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
                  {form.formState.errors.sourceUrl.message}
                </p>
              )}
              <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("subscription:edit.source_url_hint")}
              </p>
            </div>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-tags">{t("subscription:edit.tags_label")}</Label>
            <Controller
              name="tags"
              control={form.control}
              render={({ field }) => (
                <SubTagInput
                  id="edit-tags"
                  value={field.value}
                  onChange={field.onChange}
                />
              )}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-remark">
              {t("subscription:edit.remark_label")}
            </Label>
            <Input
              id="edit-remark"
              placeholder={t("subscription:edit.remark_placeholder")}
              {...form.register("remark")}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-sync-interval">
              {t("subscription:edit.sync_interval_label")}
            </Label>
            <Input
              id="edit-sync-interval"
              type="number"
              min={0}
              {...form.register("syncInterval", { valueAsNumber: true })}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-ua">{t("subscription:edit.ua_label")}</Label>
            <Input id="edit-ua" {...form.register("ua")} />
          </div>

          {subscription.type === "url" && (
            <div className="flex flex-col gap-2">
              <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
                <Controller
                  name="allowInsecure"
                  control={form.control}
                  render={({ field }) => (
                    <Checkbox
                      checked={field.value}
                      onCheckedChange={(c) => field.onChange(c === true)}
                    />
                  )}
                />
                {t("subscription:edit.allow_insecure_label")}
              </label>
              <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("subscription:edit.allow_insecure_hint")}
              </p>
            </div>
          )}

          <DialogFooter className="mt-2">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={update.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={update.isPending}>
              {t("subscription:edit.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
