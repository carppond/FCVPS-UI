"widget";
// Home-screen traffic widget (method 甲: the app pushes data; the widget only
// renders props). UI uses @expo/ui/swift-ui components — regular react-native
// View/Text are NOT available in the widget runtime. The `name` here MUST match
// the `name` in app.json's expo-widgets `widgets[]` entry ("ShiguangTraffic").
//
// The app holds the returned `trafficWidget` handle and calls
// updateSnapshot()/updateTimeline() to feed it (see lib/widget-sync.ts).
import { createWidget } from "expo-widgets";
import { VStack, HStack, Text, ProgressView, Spacer, Divider } from "@expo/ui/swift-ui";

/** Props pushed from the app. Values are pre-formatted strings so the widget
 * stays a dumb renderer (byte formatting happens app-side). */
export interface TrafficWidgetProps {
  used: string; // e.g. "1.2 TB"
  limit: string; // e.g. "2 TB" or "" when no limit
  percent: number; // 0..100
  top: { name: string; used: string }[]; // up to 3
  updatedAt: string; // e.g. "14:05"
}

const DEFAULT_PROPS: TrafficWidgetProps = {
  used: "—",
  limit: "",
  percent: 0,
  top: [],
  updatedAt: "",
};

export const trafficWidget = createWidget<TrafficWidgetProps>(
  "ShiguangTraffic",
  (props) => {
    const p = { ...DEFAULT_PROPS, ...props };
    return (
      <VStack spacing={6}>
        <HStack>
          <Text>本月流量</Text>
          <Spacer />
          <Text>{p.limit ? `${p.used} / ${p.limit}` : p.used}</Text>
        </HStack>
        <ProgressView value={Math.max(0, Math.min(1, p.percent / 100))} />
        <Divider />
        {p.top.map((a) => (
          <HStack key={a.name}>
            <Text>{a.name}</Text>
            <Spacer />
            <Text>{a.used}</Text>
          </HStack>
        ))}
        {p.updatedAt ? <Text>更新于 {p.updatedAt}</Text> : <Text> </Text>}
      </VStack>
    );
  },
);
