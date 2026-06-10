import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Loader2, Save, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "@/components/ui/toast";
import { useApiError, formatApiError } from "@/hooks/use-api-error";
import { ScriptEditor } from "@/components/script/script-editor";
import { ScriptTestPanel } from "@/components/script/script-test-panel";
import {
  useDeleteScript,
  useScript,
  useTestScript,
  useUpdateScript,
  type ScriptTestResult,
} from "@/api/script";
import i18n from "@/lib/i18n";
import scriptZh from "@/locales/zh-CN/script.json";
import scriptEn from "@/locales/en/script.json";
import scriptJa from "@/locales/ja/script.json";
import scriptKo from "@/locales/ko/script.json";
import { formatDate } from "@/lib/format";

function ensureScriptNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "script")) {
    i18n.addResourceBundle("zh-CN", "script", scriptZh, true, true);
    i18n.addResourceBundle("en", "script", scriptEn, true, true);
    i18n.addResourceBundle("ja", "script", scriptJa, true, true);
    i18n.addResourceBundle("ko", "script", scriptKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/scripts/$scriptId")({
  beforeLoad: () => {
    ensureScriptNamespace();
  },
  component: ScriptDetailPage,
});

function ScriptDetailPage() {
  const { t } = useTranslation(["script", "common"]);
  const { scriptId } = Route.useParams();
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();

  const { data: script, isLoading, isError, error, refetch } = useScript(scriptId);
  const updateMutation = useUpdateScript();
  const deleteMutation = useDeleteScript();
  const testMutation = useTestScript();

  const [name, setName] = React.useState("");
  const [code, setCode] = React.useState("");
  const [enabled, setEnabled] = React.useState(true);
  const [dirty, setDirty] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);

  // Local-only test state. Input survives a save / hot-reload because users
  // typically iterate on the same payload while tweaking the script.
  const [testInputRaw, setTestInputRaw] = React.useState("{}");
  const [testResult, setTestResult] = React.useState<
    ScriptTestResult | undefined
  >(undefined);
  const [inputError, setInputError] = React.useState<string | undefined>(
    undefined,
  );

  // Sync from server when the script loads or after a successful save.
  React.useEffect(() => {
    if (!script) return;
    setName(script.name);
    setCode(script.code);
    setEnabled(script.enabled);
    setDirty(false);
  }, [script]);

  const handleSave = async () => {
    if (!script) return;
    try {
      await updateMutation.mutateAsync({
        id: script.id,
        payload: { name, code, enabled },
      });
      toast.success(t("script:toast.saved"));
      setDirty(false);
    } catch (err) {
      handleError(err);
    }
  };

  const handleRunTest = async () => {
    if (!script) return;
    let parsed: unknown;
    if (testInputRaw.trim() === "") {
      parsed = {};
    } else {
      try {
        parsed = JSON.parse(testInputRaw);
      } catch (e) {
        setInputError(
          `${t("script:test_panel.invalid_json")}: ${
            e instanceof Error ? e.message : String(e)
          }`,
        );
        return;
      }
    }
    setInputError(undefined);
    try {
      const result = await testMutation.mutateAsync({
        id: script.id,
        input: parsed,
      });
      setTestResult(result);
      if (result.error) {
        toast.error(t("script:toast.test_failed"));
      }
    } catch (err) {
      handleError(err);
    }
  };

  const handleDelete = async () => {
    if (!script) return;
    try {
      await deleteMutation.mutateAsync(script.id);
      toast.success(t("script:toast.deleted"));
      navigate({ to: "/scripts" });
    } catch (err) {
      handleError(err);
    }
  };

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-[60vh] w-full" />
      </div>
    );
  }
  if (isError || !script) {
    return (
      <ErrorState
        message={
          formatApiError(error, t)
        }
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  return (
    <div className="flex h-[calc(100vh-7rem)] flex-col">
      <header className="flex items-center justify-between gap-3 border-b border-[var(--color-border)] pb-3">
        <div className="flex items-center gap-3">
          <Button asChild variant="ghost" size="sm">
            <Link to="/scripts">
              <ArrowLeft className="mr-1 h-3.5 w-3.5" />
              {t("script:editor.back_to_list")}
            </Link>
          </Button>
          <Input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setDirty(true);
            }}
            className="h-8 w-72"
            placeholder={t("script:editor.name_placeholder")}
          />
          <Badge variant={script.hook === "pre_save_nodes" ? "default" : "secondary"}>
            {t(`script:hook.${script.hook}`)}
          </Badge>
          <span
            className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]"
            title={t("script:editor.hook_immutable")}
          >
            *
          </span>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => {
                setEnabled(e.target.checked);
                setDirty(true);
              }}
              className="h-4 w-4 accent-[var(--color-primary)]"
            />
            {t("script:editor.enabled")}
          </label>
          <Button
            size="sm"
            onClick={handleSave}
            disabled={!dirty || updateMutation.isPending}
          >
            {updateMutation.isPending ? (
              <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Save className="mr-1 h-3.5 w-3.5" />
            )}
            {updateMutation.isPending
              ? t("script:editor.saving")
              : t("script:editor.save")}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setDeleteOpen(true)}
            disabled={deleteMutation.isPending}
          >
            <Trash2 className="mr-1 h-3.5 w-3.5" />
            {t("script:editor.delete")}
          </Button>
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="min-h-0 flex-1">
            <ScriptEditor
              value={code}
              onChange={(v) => {
                setCode(v);
                setDirty(true);
              }}
              testId="script-code-editor"
            />
          </div>

          {/* Bottom rail: last error (read-only). Falls back to "no error". */}
          <footer className="border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-4 py-2 text-[var(--font-size-xs)]">
            <p className="font-semibold uppercase tracking-wide text-[var(--color-text-tertiary)]">
              {t("script:editor.last_error_title")}
            </p>
            {script.last_error ? (
              <p
                className="mt-1 break-all text-[var(--color-error)]"
                role="alert"
              >
                {script.last_error}
                {script.last_run_at ? ` · ${formatDate(script.last_run_at)}` : ""}
              </p>
            ) : (
              <p className="mt-1 text-[var(--color-text-tertiary)]">
                {t("script:editor.no_last_error")}
              </p>
            )}
          </footer>
        </div>

        <ScriptTestPanel
          input={testInputRaw}
          onInputChange={(v) => {
            setTestInputRaw(v);
            setInputError(undefined);
          }}
          result={testResult}
          isRunning={testMutation.isPending}
          onRun={handleRunTest}
          localError={inputError}
        />
      </div>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("script:list.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("script:list.delete_confirm.description", { name })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteOpen(false)}
              disabled={deleteMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {t("script:list.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
