// Home-screen traffic widget (method 甲: the app pushes data; the widget only
// renders props). UI uses @expo/ui/swift-ui components — regular react-native
// View/Text are NOT available in the widget runtime. The `name` here MUST match
// the `name` in app.config.js's expo-widgets `widgets[]` entry ("ShiguangTraffic").
//
// Style: 极简浅色 (minimal). Background follows the system color scheme
// (env.colorScheme); one layout adapts across systemSmall / Medium / Large via
// env.widgetFamily.
//
// IMPORTANT: the `'widget'` directive must be the FIRST statement INSIDE the
// layout function body (not at module top) — babel-preset-expo's widgets-plugin
// only transforms functions whose body opens with it. Misplacing it yields a
// runtime "no layout found" error. Also: only this function body is serialized
// into the widget runtime, so it must be self-contained (no module-level vars/
// helpers) — only injected @expo/ui globals + React + JS builtins are available.
//
// The app holds the returned `trafficWidget` handle and calls
// updateSnapshot()/updateTimeline() to feed it (see lib/widget-sync.ts).
import { createWidget } from "expo-widgets";
import { VStack, HStack, Text, ProgressView, Spacer } from "@expo/ui/swift-ui";
import {
  font,
  foregroundStyle,
  tint,
  padding,
  lineLimit,
  containerBackground,
} from "@expo/ui/swift-ui/modifiers";

/** Props pushed from the app. Headline values are pre-formatted strings; agent
 * usages carry raw bytes too so the widget can size-slice the list and format a
 * "其余 N 个" remainder per family. */
export interface TrafficWidgetProps {
  used: string; // total used, formatted e.g. "1.2 TB"
  limit: string; // "2 TB" or "" when no limit
  percent: number; // 0..100
  count: number; // total VPS / agent count
  totalUsedBytes: number; // raw total, to compute the remainder line
  top: { name: string; used: string; usedBytes: number }[]; // sorted desc, up to 6
  updatedAt: string; // e.g. "14:05"
  stale?: boolean; // set by a future timeline entry once data may be outdated
}

export const trafficWidget = createWidget<TrafficWidgetProps>(
  "ShiguangTraffic",
  (props, env) => {
    "widget";
    const used = props.used || "—";
    const limit = props.limit || "";
    const percent = props.percent || 0;
    const count = props.count || 0;
    const totalUsedBytes = props.totalUsedBytes || 0;
    const top = props.top || [];
    const updatedAt = props.updatedAt || "";
    const stale = props.stale || false;

    const family = (env && env.widgetFamily) || "systemMedium";
    const dark = env && env.colorScheme === "dark";
    const bg = dark ? "#1c1c1e" : "#ffffff";
    const secondary = foregroundStyle({ type: "hierarchical", style: "secondary" });
    // Usage-based accent: green < 80%, amber < 95%, red otherwise (brand #ff6363).
    const accent = percent >= 95 ? "#ff6363" : percent >= 80 ? "#f59e0b" : "#22c55e";

    // Self-contained byte formatter for the "其余" remainder (no app helpers here).
    const fmt = (b: number): string => {
      if (!b || b < 0) return "0 B";
      const u = ["B", "KB", "MB", "GB", "TB", "PB"];
      let v = b;
      let i = 0;
      while (v >= 1024 && i < u.length - 1) {
        v /= 1024;
        i++;
      }
      return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${u[i]}`;
    };

    const bar = (
      <ProgressView value={Math.max(0, Math.min(1, percent / 100))} modifiers={[tint(accent)]} />
    );

    // ---- systemSmall: total + percent + count only, no per-agent list ----
    if (family === "systemSmall") {
      return (
        <VStack
          alignment="leading"
          spacing={3}
          modifiers={[padding({ horizontal: 14, vertical: 14 }), containerBackground(bg, "widget")]}
        >
          <Text modifiers={[font({ textStyle: "caption", weight: "semibold" }), secondary, lineLimit(1)]}>
            本月流量
          </Text>
          <Text modifiers={[font({ size: 26, weight: "bold" }), lineLimit(1)]}>{used}</Text>
          <Text modifiers={[font({ textStyle: "caption2" }), secondary, lineLimit(1)]}>
            {limit ? `共 ${limit}` : `${count} 台`}
          </Text>
          {bar}
          <Spacer />
          <HStack>
            <Text modifiers={[font({ textStyle: "caption", weight: "semibold" }), foregroundStyle(accent)]}>
              {`${Math.round(percent)}%`}
            </Text>
            <Spacer />
            <Text modifiers={[font({ textStyle: "caption2" }), secondary]}>{`${count} 台`}</Text>
          </HStack>
        </VStack>
      );
    }

    // ---- systemMedium (Top 3) / systemLarge (Top 6) share one structure ----
    const shownK = family === "systemLarge" ? 6 : 3;
    const shown = top.slice(0, shownK);
    let shownBytes = 0;
    for (const a of shown) shownBytes += a.usedBytes || 0;
    const restCount = Math.max(0, count - shown.length);
    const restBytes = Math.max(0, totalUsedBytes - shownBytes);
    const heroSize = family === "systemLarge" ? 28 : 22;

    // Build the agent rows as an array (array children are the proven-safe
    // pattern — avoids null/conditional element children in the widget runtime).
    const rows = shown.map((a) => (
      <HStack key={a.name}>
        <Text modifiers={[font({ textStyle: "footnote" }), secondary, lineLimit(1)]}>{a.name}</Text>
        <Spacer />
        <Text modifiers={[font({ textStyle: "footnote" }), lineLimit(1)]}>{a.used}</Text>
      </HStack>
    ));
    if (restCount > 0) {
      rows.push(
        <HStack key="__rest">
          <Text modifiers={[font({ textStyle: "footnote" }), secondary, lineLimit(1)]}>
            {`其余 ${restCount} 个`}
          </Text>
          <Spacer />
          <Text modifiers={[font({ textStyle: "footnote" }), secondary, lineLimit(1)]}>
            {fmt(restBytes)}
          </Text>
        </HStack>,
      );
    }

    return (
      <VStack
        alignment="leading"
        spacing={family === "systemLarge" ? 8 : 6}
        modifiers={[padding({ horizontal: 16, vertical: 14 }), containerBackground(bg, "widget")]}
      >
        <HStack>
          <Text modifiers={[font({ textStyle: "subheadline", weight: "semibold" }), secondary, lineLimit(1)]}>
            本月流量
          </Text>
          <Spacer />
          <Text modifiers={[font({ textStyle: "caption2" }), secondary, lineLimit(1)]}>
            {count > 0 ? `共 ${count} 台 · ${updatedAt}` : updatedAt}
          </Text>
        </HStack>
        <HStack>
          <Text modifiers={[font({ size: heroSize, weight: "bold" }), lineLimit(1)]}>{used}</Text>
          <Text modifiers={[font({ textStyle: "footnote" }), secondary]}>{limit ? ` / ${limit}` : ""}</Text>
          <Spacer />
          <Text modifiers={[font({ textStyle: "callout", weight: "semibold" }), foregroundStyle(accent)]}>
            {`${Math.round(percent)}%`}
          </Text>
        </HStack>
        {bar}
        {rows}
        <Spacer />
        {stale ? (
          <Text modifiers={[font({ textStyle: "caption2" }), foregroundStyle("#f59e0b"), lineLimit(1)]}>
            {`更新 ${updatedAt} · 可能过期`}
          </Text>
        ) : (
          <Text modifiers={[font({ textStyle: "caption2" }), secondary, lineLimit(1)]}>
            {updatedAt ? `更新 ${updatedAt}` : " "}
          </Text>
        )}
      </VStack>
    );
  },
);
