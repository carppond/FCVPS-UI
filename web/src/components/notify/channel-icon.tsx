import {
  Bell,
  Globe,
  Hash,
  Mail,
  MessageCircle,
  MessageSquare,
  Send,
  Smartphone,
  Webhook,
  Zap,
} from "lucide-react";
import type { ChannelKind } from "@/types/api";
import { cn } from "@/lib/cn";

/**
 * Single-color lucide icon mapped per ChannelKind. We deliberately use
 * monochrome icons (per the design system's "monochrome + single accent"
 * rule) rather than each vendor's brand colour — vendor logos clash with
 * the data-ink-first chart palette and break theme switching.
 */
const ICON_MAP: Record<ChannelKind, React.ComponentType<{ className?: string }>> = {
  telegram: Send,
  discord: MessageCircle,
  slack: Hash,
  email: Mail,
  bark: Smartphone,
  gotify: Bell,
  webhook: Webhook,
  serverchan: MessageSquare,
  pushdeer: Smartphone,
  ifttt: Zap,
};

const FALLBACK = Globe;

interface ChannelIconProps {
  kind: ChannelKind;
  className?: string;
}

export function ChannelIcon({ kind, className }: ChannelIconProps) {
  const Icon = ICON_MAP[kind] ?? FALLBACK;
  return <Icon className={cn("h-5 w-5", className)} />;
}
