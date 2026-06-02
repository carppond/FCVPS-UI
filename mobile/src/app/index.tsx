import { Redirect } from "expo-router";
import { ActivityIndicator, View } from "react-native";
import { useAuthStore } from "../stores/auth-store";
import { type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import { useMemo } from "react";

export default function Index() {
  const colors = useColors();
  const { token, isReady } = useAuthStore();

  if (!isReady) {
    return (
      <View style={{ flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: colors.bg }}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  if (token) {
    return <Redirect href="/(tabs)" />;
  }

  return <Redirect href="/(auth)/login" />;
}
