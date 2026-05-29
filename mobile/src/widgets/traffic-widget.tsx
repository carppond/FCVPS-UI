// Home-screen traffic widget (method 甲: the app pushes data; the widget only
// renders props). UI uses @expo/ui/swift-ui components — regular react-native
// View/Text are NOT available in the widget runtime. The `name` here MUST match
// the `name` in app.json's expo-widgets `widgets[]` entry ("ShiguangTraffic").
//
// IMPORTANT: the `'widget'` directive must be the FIRST statement INSIDE the
// layout function body (not at module top) — babel-preset-expo's widgets-plugin
// only transforms functions whose body opens with it. Misplacing it yields a
// runtime "no layout found" error.
//
// The app holds the returned `trafficWidget` handle and calls
// updateSnapshot()/updateTimeline() to feed it (see lib/widget-sync.ts).
import { createWidget } from "expo-widgets";
import { VStack, HStack, Text, ProgressView, Spacer, Divider } from "@expo/ui/swift-ui";
import { font, foregroundStyle, tint, padding } from "@expo/ui/swift-ui/modifiers";

/** Props pushed from the app. Values are pre-formatted strings so the widget
 * stays a dumb renderer (byte formatting happens app-side). */
export interface TrafficWidgetProps {
  used: string; // e.g. "1.2 TB"
  limit: string; // e.g. "2 TB" or "" when no limit
  percent: number; // 0..100
  top: { name: string; used: string }[]; // up to 3
  updatedAt: string; // e.g. "14:05"
}

export const trafficWidget = createWidget<TrafficWidgetProps>(
  "ShiguangTraffic",
  (props) => {
    "widget";
    // Everything the widget renders must live INSIDE this function — the
    // 'widget' directive serializes only the function body to the widget
    // runtime; module-level vars/helpers are NOT available there (only the
    // injected @expo/ui/swift-ui globals + React + JS builtins).
    // Per-field fallback (widget may render with empty props before the app
    // pushes data). No object-spread defaults — props is a complete type so a
    // spread would just overwrite them (TS2783).
    const used = props.used || "—";
    const limit = props.limit || "";
    const percent = props.percent || 0;
    const top = props.top || [];
    const updatedAt = props.updatedAt || "";
    // Usage-based bar color: green < 80%, amber < 95%, red otherwise (red is the
    // app brand #ff6363). Styling uses @expo/ui/swift-ui/modifiers, which the
    // widget bundle injects as globals (same as the components).
    const barColor = percent >= 95 ? "#ff6363" : percent >= 80 ? "#f59e0b" : "#22c55e";
    const secondary = foregroundStyle({ type: "hierarchical", style: "secondary" });
    return (
      <VStack alignment="leading" spacing={6} modifiers={[padding({ horizontal: 14, vertical: 12 })]}>
        <HStack>
          <Text modifiers={[font({ textStyle: "subheadline", weight: "semibold" }), secondary]}>
            本月流量
          </Text>
          <Spacer />
          <Text modifiers={[font({ weight: "semibold" })]}>
            {limit ? `${used} / ${limit}` : used}
          </Text>
        </HStack>
        <ProgressView value={Math.max(0, Math.min(1, percent / 100))} modifiers={[tint(barColor)]} />
        <Divider />
        {top.map((a) => (
          <HStack key={a.name}>
            <Text modifiers={[font({ textStyle: "footnote" }), secondary]}>{a.name}</Text>
            <Spacer />
            <Text modifiers={[font({ textStyle: "footnote" })]}>{a.used}</Text>
          </HStack>
        ))}
        {updatedAt ? (
          <Text modifiers={[font({ textStyle: "caption2" }), secondary]}>更新于 {updatedAt}</Text>
        ) : (
          <Text> </Text>
        )}
      </VStack>
    );
  },
);
