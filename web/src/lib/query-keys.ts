/**
 * Centralized TanStack Query key factory.
 * Keys are structured arrays for fine-grained invalidation.
 *
 * Usage:
 *   useQuery({ queryKey: queryKeys.subscription.detail("sub_123") })
 *   queryClient.invalidateQueries({ queryKey: queryKeys.subscription.all() })
 */
export const queryKeys = {
  user: {
    all: () => ["user"] as const,
    list: () => ["user", "list"] as const,
    detail: (id: string) => ["user", "detail", id] as const,
    me: () => ["user", "me"] as const,
  },

  subscription: {
    all: () => ["subscription"] as const,
    list: () => ["subscription", "list"] as const,
    detail: (id: string) => ["subscription", "detail", id] as const,
    tags: () => ["subscription", "tags"] as const,
  },

  node: {
    all: () => ["node"] as const,
    list: (subscriptionId?: string) => ["node", "list", subscriptionId] as const,
    detail: (id: string) => ["node", "detail", id] as const,
  },

  pipeline: {
    all: () => ["pipeline"] as const,
    list: () => ["pipeline", "list"] as const,
    detail: (id: string) => ["pipeline", "detail", id] as const,
  },

  rule: {
    all: () => ["rule"] as const,
    list: () => ["rule", "list"] as const,
    detail: (id: string) => ["rule", "detail", id] as const,
  },

  ruleSet: {
    all: () => ["rule-set"] as const,
    list: () => ["rule-set", "list"] as const,
    detail: (id: string) => ["rule-set", "detail", id] as const,
    presets: () => ["rule-set", "presets"] as const,
  },

  proxyGroup: {
    all: () => ["proxy-group"] as const,
    list: () => ["proxy-group", "list"] as const,
    detail: (id: string) => ["proxy-group", "detail", id] as const,
    presets: () => ["proxy-group", "presets"] as const,
  },

  script: {
    all: () => ["script"] as const,
    list: () => ["script", "list"] as const,
    detail: (id: string) => ["script", "detail", id] as const,
  },

  agent: {
    all: () => ["agent"] as const,
    list: () => ["agent", "list"] as const,
    detail: (id: string) => ["agent", "detail", id] as const,
    metrics: (id: string) => ["agent", "metrics", id] as const,
  },

  traffic: {
    all: () => ["traffic"] as const,
    summary: () => ["traffic", "summary"] as const,
    byAgent: (agentId: string) => ["traffic", "agent", agentId] as const,
    bySubscription: (subId: string) => ["traffic", "subscription", subId] as const,
  },

  notify: {
    all: () => ["notify"] as const,
    channels: () => ["notify", "channels"] as const,
    channel: (id: string) => ["notify", "channel", id] as const,
    events: () => ["notify", "events"] as const,
  },

  vpsAsset: {
    all: () => ["vps-asset"] as const,
    list: () => ["vps-asset", "list"] as const,
    detail: (id: string) => ["vps-asset", "detail", id] as const,
    summary: () => ["vps-asset", "summary"] as const,
  },
} as const;
