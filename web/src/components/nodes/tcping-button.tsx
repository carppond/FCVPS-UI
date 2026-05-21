import { Activity, Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useTCPingNodeMutation } from "@/api/node";

/**
 * Single-node TCPing trigger. Shows a spinner while the mutation is in flight
 * and surfaces success / failure via the global toast system.
 *
 * The button is intentionally minimal — protocol-specific behaviour belongs in
 * the parent (e.g. disabling for chain-only nodes).
 */
interface TCPingButtonProps {
  nodeId: string;
  size?: "default" | "sm";
}

export function TCPingButton({ nodeId, size = "sm" }: TCPingButtonProps) {
  const { t } = useTranslation("node");
  const { handle: handleError } = useApiError();
  const mutation = useTCPingNodeMutation();

  const probe = async () => {
    try {
      const res = await mutation.mutateAsync(nodeId);
      if (res.reachable) {
        toast.success(`${res.latency_ms} ms`);
      } else {
        toast.error(t("latency.unreachable"));
      }
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <Button
      variant="outline"
      size={size}
      onClick={probe}
      disabled={mutation.isPending}
      aria-label={t("actions.tcping_one")}
    >
      {mutation.isPending ? (
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
      ) : (
        <Activity className="h-3.5 w-3.5" />
      )}
      {t("actions.tcping_one")}
    </Button>
  );
}
