import * as React from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import { ArrowLeft, Code2, Play, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError, formatApiError } from "@/hooks/use-api-error";
import { OperatorLibrary } from "@/components/pipeline/operator-library";
import {
  PipelineCanvas,
  resolveCanvasDragEnd,
} from "@/components/pipeline/canvas";
import { ParamPanel } from "@/components/pipeline/param-panel";
import { YamlPane } from "@/components/pipeline/yaml-pane";
import { PreviewPane } from "@/components/pipeline/preview-pane";
import {
  usePipeline,
  useToYaml,
  useUpdatePipeline,
} from "@/api/pipeline";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import { cn } from "@/lib/cn";
import type { OperatorType, Pipeline } from "@/types/api";

export const Route = createFileRoute("/_authed/pipelines/$pipelineId/edit")({
  component: PipelineEditorPage,
});

function PipelineEditorPage() {
  const { pipelineId } = Route.useParams();
  const navigate = useNavigate();
  const { t } = useTranslation(["pipeline", "common"]);
  const { handle: handleError } = useApiError();

  const { data, isLoading, isError, error, refetch } = usePipeline(pipelineId);

  // Editor store actions / state.
  const loadPipeline = usePipelineEditorStore((s) => s.loadPipeline);
  const setName = usePipelineEditorStore((s) => s.setName);
  const addOperator = usePipelineEditorStore((s) => s.addOperator);
  const reorderOperators = usePipelineEditorStore((s) => s.reorderOperators);
  const selectOperator = usePipelineEditorStore((s) => s.selectOperator);
  const getSnapshot = usePipelineEditorStore((s) => s.getPipelineSnapshot);
  const markClean = usePipelineEditorStore((s) => s.markClean);
  const name = usePipelineEditorStore((s) => s.name);
  const dirty = usePipelineEditorStore((s) => s.dirty);
  const operators = usePipelineEditorStore((s) => s.ast.operators);

  // Hydrate the store once when the pipeline detail resolves; subsequent
  // re-renders should keep the user's local edits unless the id changes.
  React.useEffect(() => {
    if (data) loadPipeline(data);
  }, [data, loadPipeline]);

  // Reset selection when leaving the page so a stale id doesn't bleed into
  // the next pipeline opened.
  React.useEffect(
    () => () => {
      selectOperator(null);
    },
    [selectOperator],
  );

  const toYamlMutation = useToYaml();
  const updateMutation = useUpdatePipeline();

  // ── Drag-end ──────────────────────────────────────────────────────────────
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
  );

  const handleDragEnd = React.useCallback(
    (event: DragEndEvent) => {
      const activeId = String(event.active.id);
      const overId = event.over ? String(event.over.id) : null;
      const resolution = resolveCanvasDragEnd({
        activeId,
        overId,
        operators,
        store: { addOperator, reorderOperators },
      });
      if (resolution.kind === "add" && resolution.newOperatorId) {
        selectOperator(resolution.newOperatorId);
      }
    },
    [operators, addOperator, reorderOperators, selectOperator],
  );

  // ── Save ──────────────────────────────────────────────────────────────────
  const handleSave = async () => {
    const snap = getSnapshot();
    try {
      // Server is the source of truth for YAML serialization: send AST →
      // ast-to-yaml → use the canonical YAML in the PUT body.
      const astJson = JSON.stringify(snap.ast);
      const { yaml_content } = await toYamlMutation.mutateAsync(astJson);
      const saved: Pipeline = await updateMutation.mutateAsync({
        id: pipelineId,
        payload: {
          name: snap.name,
          yaml_content,
          ast_json: astJson,
          version: snap.version,
        },
      });
      markClean({ version: saved.version, yamlContent: saved.yaml_content });
      toast.success(t("pipeline:toast.saved"));
    } catch (err) {
      handleError(err);
    }
  };

  // ── Right-rail mode + debug preview dialog ────────────────────────────────
  const [rightRail, setRightRail] = React.useState<"param" | "yaml">("param");
  const [previewOpen, setPreviewOpen] = React.useState(false);

  // ── Render ────────────────────────────────────────────────────────────────
  if (isLoading) return <EditorSkeleton />;
  if (isError || !data) {
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

  const handleLibraryClickAdd = (type: OperatorType) => {
    const id = addOperator(type);
    selectOperator(id);
  };

  return (
    <div className="flex h-[calc(100vh-var(--spacing-24))] flex-col gap-3">
      {/* Top bar */}
      <header className="flex items-center gap-3 px-1">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate({ to: "/pipelines" })}
        >
          <ArrowLeft className="mr-1 h-4 w-4" />
          {t("pipeline:editor.back")}
        </Button>

        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("pipeline:editor.name_placeholder")}
          aria-label={t("pipeline:editor.name_placeholder")}
          className="h-9 max-w-xs"
        />

        <span
          aria-live="polite"
          className={cn(
            "flex items-center gap-1.5 text-[var(--font-size-xs)]",
            dirty
              ? "text-[var(--color-warning)]"
              : "text-[var(--color-text-tertiary)]",
          )}
        >
          {dirty && (
            <span
              className="inline-block h-1.5 w-1.5 rounded-full bg-[var(--color-warning)]"
              aria-hidden
            />
          )}
          {dirty
            ? t("pipeline:editor.dirty_indicator")
            : t("pipeline:editor.saved")}
        </span>

        <div className="ml-auto flex items-center gap-2">
          <Button
            variant={rightRail === "yaml" ? "default" : "outline"}
            size="sm"
            onClick={() =>
              setRightRail((prev) => (prev === "yaml" ? "param" : "yaml"))
            }
            data-testid="toggle-yaml"
          >
            <Code2 className="mr-1 h-4 w-4" />
            {t("pipeline:editor.toggle_yaml")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPreviewOpen(true)}
            data-testid="open-preview"
          >
            <Play className="mr-1 h-4 w-4" />
            {t("pipeline:editor.debug_preview")}
          </Button>
          <Button
            size="sm"
            onClick={handleSave}
            disabled={updateMutation.isPending || toYamlMutation.isPending}
          >
            <Save className="mr-1 h-4 w-4" />
            {updateMutation.isPending || toYamlMutation.isPending
              ? t("pipeline:editor.saving")
              : t("pipeline:editor.save")}
          </Button>
        </div>
      </header>

      {/* 3-column editor */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <div className="flex min-h-0 flex-1 overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-bg)]">
          <OperatorLibrary onClickAdd={handleLibraryClickAdd} />
          <PipelineCanvas />
          {rightRail === "yaml" ? (
            <YamlPane onBackToParam={() => setRightRail("param")} />
          ) : (
            <ParamPanel />
          )}
        </div>
      </DndContext>

      <PreviewPane
        open={previewOpen}
        onOpenChange={setPreviewOpen}
        pipelineId={pipelineId}
      />
    </div>
  );
}

function EditorSkeleton() {
  return (
    <div className="flex h-[calc(100vh-var(--spacing-24))] flex-col gap-3">
      <header className="flex items-center gap-3">
        <Skeleton className="h-8 w-24" />
        <Skeleton className="h-8 w-48" />
        <Skeleton className="ml-auto h-8 w-24" />
        <Skeleton className="h-8 w-24" />
      </header>
      <div className="flex min-h-0 flex-1 gap-3">
        <Skeleton className="h-full w-60" />
        <Skeleton className="h-full flex-1" />
        <Skeleton className="h-full w-80" />
      </div>
    </div>
  );
}
