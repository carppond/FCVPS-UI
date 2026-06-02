import { Stack } from "expo-router";
import { type AppColors } from "../../lib/theme";
import { useColors } from "../../lib/useColors";
import { useMemo } from "react";

export default function AuthLayout() {
  const colors = useColors();
  return (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: colors.bg },
      }}
    />
  );
}
