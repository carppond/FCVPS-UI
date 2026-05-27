import * as React from "react";
import { useTranslation } from "react-i18next";
import { Loader2, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { useTestChannel } from "@/api/notify";
import { useApiError } from "@/hooks/use-api-error";

interface ChannelTestButtonProps {
  channelId: string;
  /**
   * Optional callback fired after the test completes. The notifications
   * page uses this to drive the per-channel "ok / failed" status badge.
   */
  onResult?: (ok: boolean, error?: string) => void;
}

/**
 * Test-send button. Disabled while in-flight; shows a spinner and toasts
 * the result. The button is intentionally short — the surrounding card
 * supplies the channel context (name / kind) so we don't repeat it here.
 */
export function ChannelTestButton({
  channelId,
  onResult,
}: ChannelTestButtonProps) {
  const { t } = useTranslation(["notify", "common"]);
  const { handle: handleError } = useApiError();
  const testMutation = useTestChannel();

  const handleClick = React.useCallback(async () => {
    try {
      const res = await testMutation.mutateAsync(channelId);
      if (!res || res.ok) {
        toast.success(t("notify:test.success"));
        onResult?.(true);
      } else {
        const err = res.error ?? t("notify:test.unknown_error");
        toast.error(t("notify:test.failed_with_reason", { reason: err }));
        onResult?.(false, err);
      }
    } catch (err) {
      handleError(err);
      onResult?.(
        false,
        err instanceof Error ? err.message : String(err ?? ""),
      );
    }
  }, [channelId, testMutation, onResult, t, handleError]);

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={testMutation.isPending}
      onClick={handleClick}
      data-testid={`notify-channel-test-${channelId}`}
    >
      {testMutation.isPending ? (
        <Loader2 className="h-4 w-4 animate-spin" />
      ) : (
        <Send className="h-4 w-4" />
      )}
      {t("notify:test.button")}
    </Button>
  );
}
