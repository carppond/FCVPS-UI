// Dynamic Expo config — makes the iOS traffic widget OPT-IN at build time.
//
// Why: the widget uses expo-widgets + an App Group, which require a PAID Apple
// Developer account. If the widget were always in the config, anyone without a
// paid account (most open-source users) could not even build the base app
// (App Group provisioning fails). So by default we exclude it, and gate it
// behind the EXPO_WIDGET env var.
//
//   npx expo prebuild -p ios --clean              # base build — no widget, free account OK
//   EXPO_WIDGET=1 npx expo prebuild -p ios --clean  # widget build — needs paid account
//
// Same switch works for EAS via a build profile env (see eas.json).
//
// `config` is loaded from app.json; we only append the widget plugin here.

const widgetPlugin = [
  "expo-widgets",
  {
    groupIdentifier: "group.com.shiguang.vps",
    enablePushNotifications: false,
    widgets: [
      {
        name: "ShiguangTraffic",
        displayName: "拾光VPS 流量",
        description: "本月流量与 Top 探针",
        supportedFamilies: ["systemMedium"],
        contentMarginsDisabled: false,
      },
    ],
  },
];

module.exports = ({ config }) => {
  const plugins = [...(config.plugins ?? [])];
  if (process.env.EXPO_WIDGET === "1") {
    plugins.push(widgetPlugin);
  }

  // Apple Team ID is a real account identifier — keep it OUT of the committed
  // app.json (public repo). Inject from env for CLI builds; Xcode GUI builds
  // can just pick the team in Signing & Capabilities instead.
  const ios = { ...(config.ios ?? {}) };
  if (process.env.APPLE_TEAM_ID) {
    ios.appleTeamId = process.env.APPLE_TEAM_ID;
  }

  return { ...config, ios, plugins };
};

