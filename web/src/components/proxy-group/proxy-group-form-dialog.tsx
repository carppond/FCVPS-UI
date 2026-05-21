import * as React from "react";
import { useTranslation } from "react-i18next";
import { Save, X } from "lucide-react";
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useCreateProxyGroup,
  useUpdateProxyGroup,
} from "@/api/proxy-group";
import type { ProxyGroupCategory, ProxyGroupType } from "@/types/api";

interface ProxyGroupFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  group?: ProxyGroupCategory | null;
}

const TEST_FILTER_QUICK = [
  { label: "HK", regex: "(?i)HK|香港|🇭🇰" },
  { label: "JP", regex: "(?i)JP|日本|🇯🇵" },
  { label: "US", regex: "(?i)US|美国|🇺🇸" },
  { label: "SG", regex: "(?i)SG|新加坡|🇸🇬" },
  { label: "TW", regex: "(?i)TW|台湾|🇹🇼" },
  { label: "KR", regex: "(?i)KR|韩国|🇰🇷" },
];

const TYPES: ProxyGroupType[] = [
  "select",
  "url-test",
  "fallback",
  "load-balance",
  "relay",
];

/**
 * Create / edit dialog for a single proxy group. Members + sub-groups are
 * captured as Enter-to-commit chip inputs; filter regex hides until the user
 * toggles `include_all` because mihomo ignores filter when it's off. The
 * url-test family (url-test / fallback / load-balance) reveals the test URL
 * + interval fields; other types hide them entirely.
 */
export function ProxyGroupFormDialog({
  open,
  onOpenChange,
  group,
}: ProxyGroupFormDialogProps) {
  const { t } = useTranslation(["proxy-group", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateProxyGroup();
  const updateMutation = useUpdateProxyGroup();

  const editing = Boolean(group);

  const [name, setName] = React.useState("");
  const [icon, setIcon] = React.useState("");
  const [type, setType] = React.useState<ProxyGroupType>("select");
  const [includeAll, setIncludeAll] = React.useState(false);
  const [filter, setFilter] = React.useState("");
  const [members, setMembers] = React.useState<string[]>([]);
  const [subGroups, setSubGroups] = React.useState<string[]>([]);
  const [testUrl, setTestUrl] = React.useState("");
  const [testInterval, setTestInterval] = React.useState<number | "">("");
  const [sortOrder, setSortOrder] = React.useState<number>(0);
  const [submitting, setSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    if (group) {
      setName(group.name);
      setIcon(group.icon ?? "");
      setType(group.type);
      setIncludeAll(group.include_all);
      setFilter(group.filter ?? "");
      setMembers(group.member_proxies ?? []);
      setSubGroups(group.member_groups ?? []);
      setTestUrl(group.test_url ?? "");
      setTestInterval(group.test_interval ?? "");
      setSortOrder(group.sort_order ?? 0);
    } else {
      setName("");
      setIcon("");
      setType("select");
      setIncludeAll(false);
      setFilter("");
      setMembers([]);
      setSubGroups([]);
      setTestUrl("");
      setTestInterval(300);
      setSortOrder(0);
    }
  }, [open, group]);

  const isUrlTestFamily =
    type === "url-test" || type === "fallback" || type === "load-balance";

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      const payload = {
        name: name.trim(),
        type,
        icon: icon.trim() || undefined,
        include_all: includeAll,
        filter: includeAll && filter.trim() ? filter.trim() : undefined,
        member_proxies: members,
        member_groups: subGroups,
        test_url: isUrlTestFamily && testUrl.trim() ? testUrl.trim() : undefined,
        test_interval:
          isUrlTestFamily && typeof testInterval === "number"
            ? testInterval
            : undefined,
        sort_order: sortOrder,
      };
      if (group) {
        await updateMutation.mutateAsync({
          id: group.id,
          payload: {
            ...payload,
            // PUT requires explicit null to clear icon / filter / test_url
            icon: payload.icon ?? null,
            filter: payload.filter ?? null,
            test_url: payload.test_url ?? null,
          },
        });
        toast.success(t("proxy-group:toast.update_ok"));
      } else {
        await createMutation.mutateAsync(payload);
        toast.success(t("proxy-group:toast.create_ok"));
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
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {editing
              ? t("proxy-group:form.edit_title")
              : t("proxy-group:form.new_title")}
          </DialogTitle>
        </DialogHeader>

        <form
          className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto pr-1"
          onSubmit={onSubmit}
        >
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_8rem]">
            <section className="flex flex-col gap-2">
              <Label htmlFor="pg-name">{t("proxy-group:form.name_label")}</Label>
              <Input
                id="pg-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("proxy-group:form.name_placeholder")}
                required
                autoComplete="off"
              />
            </section>
            <section className="flex flex-col gap-2">
              <Label htmlFor="pg-icon">{t("proxy-group:form.icon_label")}</Label>
              <Input
                id="pg-icon"
                value={icon}
                onChange={(e) => setIcon(e.target.value)}
                placeholder={t("proxy-group:form.icon_placeholder")}
                maxLength={8}
              />
            </section>
          </div>

          <section className="flex flex-col gap-2">
            <Label>{t("proxy-group:form.type_label")}</Label>
            <Tabs
              value={type}
              onValueChange={(v) => setType(v as ProxyGroupType)}
            >
              <TabsList className="w-full">
                {TYPES.map((kind) => (
                  <TabsTrigger key={kind} value={kind} className="flex-1">
                    {t(`proxy-group:types.${kind}`)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </section>

          <label className="flex cursor-pointer items-start gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            <input
              type="checkbox"
              checked={includeAll}
              onChange={(e) => setIncludeAll(e.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-[var(--color-border-strong)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
            />
            <span className="flex flex-col gap-0.5">
              <span>{t("proxy-group:form.include_all_label")}</span>
              <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {t("proxy-group:form.include_all_hint")}
              </span>
            </span>
          </label>

          {includeAll && (
            <section className="flex flex-col gap-2">
              <Label htmlFor="pg-filter">
                {t("proxy-group:form.filter_label")}
              </Label>
              <Input
                id="pg-filter"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder={t("proxy-group:form.filter_placeholder")}
                autoComplete="off"
              />
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                  {t("proxy-group:form.filter_quick")}:
                </span>
                {TEST_FILTER_QUICK.map((q) => (
                  <button
                    key={q.label}
                    type="button"
                    onClick={() => setFilter(q.regex)}
                    className={cn(
                      "rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-0.5",
                      "text-[var(--font-size-xs)] text-[var(--color-text-secondary)]",
                      "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
                    )}
                  >
                    {q.label}
                  </button>
                ))}
              </div>
            </section>
          )}

          <ChipsInput
            id="pg-members"
            label={t("proxy-group:form.members_label")}
            placeholder={t("proxy-group:form.members_placeholder")}
            values={members}
            onChange={setMembers}
          />

          <ChipsInput
            id="pg-subgroups"
            label={t("proxy-group:form.subgroups_label")}
            placeholder={t("proxy-group:form.subgroups_placeholder")}
            values={subGroups}
            onChange={setSubGroups}
          />

          {isUrlTestFamily && (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_8rem]">
              <section className="flex flex-col gap-2">
                <Label htmlFor="pg-test-url">
                  {t("proxy-group:form.test_url_label")}
                </Label>
                <Input
                  id="pg-test-url"
                  value={testUrl}
                  onChange={(e) => setTestUrl(e.target.value)}
                  placeholder={t("proxy-group:form.test_url_placeholder")}
                  type="url"
                  autoComplete="off"
                />
              </section>
              <section className="flex flex-col gap-2">
                <Label htmlFor="pg-test-interval">
                  {t("proxy-group:form.test_interval_label")}
                </Label>
                <Input
                  id="pg-test-interval"
                  type="number"
                  min={0}
                  value={testInterval}
                  onChange={(e) => {
                    const n = Number(e.target.value);
                    setTestInterval(Number.isNaN(n) ? "" : n);
                  }}
                  placeholder={t("proxy-group:form.test_interval_placeholder")}
                />
              </section>
            </div>
          )}

          <section className="flex flex-col gap-2">
            <Label htmlFor="pg-sort">
              {t("proxy-group:form.sort_order_label")}
            </Label>
            <Input
              id="pg-sort"
              type="number"
              value={sortOrder}
              onChange={(e) => setSortOrder(Number(e.target.value) || 0)}
            />
          </section>

          <DialogFooter className="sticky bottom-0 -mx-1 mt-2 border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-1 pt-3">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              {t("proxy-group:form.cancel")}
            </Button>
            <Button type="submit" size="sm" disabled={submitting}>
              <Save className="h-3.5 w-3.5" />
              {t("proxy-group:form.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

interface ChipsInputProps {
  id: string;
  label: string;
  placeholder: string;
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Lightweight chips input. Commits on Enter, comma, or blur — duplicates are
 * silently ignored. Backspace from an empty input pops the last chip so it
 * feels like the tag inputs used elsewhere in the app.
 */
function ChipsInput({
  id,
  label,
  placeholder,
  values,
  onChange,
}: ChipsInputProps) {
  const [draft, setDraft] = React.useState("");

  const commit = (raw: string) => {
    const v = raw.trim();
    if (!v) return;
    if (values.includes(v)) {
      setDraft("");
      return;
    }
    onChange([...values, v]);
    setDraft("");
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit(draft);
      return;
    }
    if (e.key === "Backspace" && draft === "" && values.length > 0) {
      onChange(values.slice(0, -1));
    }
  };

  return (
    <section className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <div
        className={cn(
          "flex min-h-9 flex-wrap items-center gap-1.5 rounded-[var(--radius-md)]",
          "border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-2 py-1.5",
          "focus-within:ring-2 focus-within:ring-[var(--color-primary)]",
          "transition-colors duration-[var(--duration-fast)]",
        )}
      >
        {values.map((v) => (
          <Badge
            key={v}
            variant="secondary"
            className="gap-1 pr-1"
          >
            <span>{v}</span>
            <button
              type="button"
              onClick={() =>
                onChange(values.filter((existing) => existing !== v))
              }
              className="inline-flex h-4 w-4 items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-hover)]"
              aria-label={`Remove ${v}`}
            >
              <X className="h-3 w-3" />
            </button>
          </Badge>
        ))}
        <input
          id={id}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          onBlur={() => commit(draft)}
          placeholder={values.length === 0 ? placeholder : ""}
          className="min-w-32 flex-1 bg-transparent text-[var(--font-size-sm)] text-[var(--color-text-primary)] placeholder:text-[var(--color-text-disabled)] focus:outline-none"
          autoComplete="off"
        />
      </div>
    </section>
  );
}
