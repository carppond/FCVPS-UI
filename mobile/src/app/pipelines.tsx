import { useState, useCallback, useMemo } from "react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
  TouchableOpacity,
  Modal,
  ScrollView,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTranslation } from "react-i18next";
import { usePipelinesQuery } from "../api/pipeline";
import { spacing, radius, fontSize, type AppColors } from "../lib/theme";
import { useColors } from "../lib/useColors";
import type { Pipeline } from "../types/api";

function formatDate(ts: number): string {
  const d = new Date(ts * 1000);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function countOperators(astJson: string): number {
  try {
    const ast = JSON.parse(astJson);
    if (Array.isArray(ast)) return ast.length;
    if (ast?.operators && Array.isArray(ast.operators)) return ast.operators.length;
    return 0;
  } catch {
    return 0;
  }
}

export default function PipelinesScreen() {
  const { t } = useTranslation("rules");
  const colors = useColors();
  const styles = useMemo(() => makeStyles(colors), [colors]);
  const { data, isLoading, refetch } = usePipelinesQuery();
  const [refreshing, setRefreshing] = useState(false);
  const [selectedPipeline, setSelectedPipeline] = useState<Pipeline | null>(
    null,
  );

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refetch();
    setRefreshing(false);
  }, []);

  const items = data?.items ?? [];

  const renderItem = ({ item }: { item: Pipeline }) => {
    const opCount = countOperators(item.ast_json);
    return (
      <TouchableOpacity
        style={styles.card}
        onPress={() => setSelectedPipeline(item)}
        activeOpacity={0.7}
      >
        <View style={styles.cardTop}>
          <View style={styles.cardInfo}>
            <Text style={styles.cardName} numberOfLines={1}>
              {item.name}
            </Text>
            <View style={styles.badgeRow}>
              <View style={styles.badge}>
                <Text style={styles.badgeText}>{t("pipeline_count_operators", { count: opCount })}</Text>
              </View>
              <View style={styles.badge}>
                <Text style={styles.badgeText}>v{item.version}</Text>
              </View>
              <Text style={styles.dateText}>
                {formatDate(item.updated_at)}
              </Text>
            </View>
          </View>
          <Ionicons
            name="chevron-forward"
            size={16}
            color={colors.textDisabled}
          />
        </View>
      </TouchableOpacity>
    );
  };

  return (
    <View style={styles.container}>
      <FlatList
        contentContainerStyle={items.length === 0 ? styles.empty : styles.list}
        data={items}
        keyExtractor={(item) => item.id}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={onRefresh}
            tintColor={colors.primary}
          />
        }
        ListEmptyComponent={
          !isLoading ? (
            <View style={styles.emptyBox}>
              <Ionicons
                name="git-merge-outline"
                size={48}
                color={colors.textDisabled}
              />
              <Text style={styles.emptyText}>{t("pipeline_empty")}</Text>
            </View>
          ) : null
        }
        renderItem={renderItem}
      />

      {/* YAML Detail Modal */}
      <Modal
        visible={selectedPipeline !== null}
        animationType="slide"
        presentationStyle="pageSheet"
        onRequestClose={() => setSelectedPipeline(null)}
      >
        <View style={styles.modalContainer}>
          <View style={styles.modalHeader}>
            <TouchableOpacity onPress={() => setSelectedPipeline(null)}>
              <Text style={styles.modalCancel}>{t("pipeline_modal_close")}</Text>
            </TouchableOpacity>
            <Text style={styles.modalTitle} numberOfLines={1}>
              {selectedPipeline?.name}
            </Text>
            <View style={{ width: 40 }} />
          </View>
          <ScrollView
            style={styles.yamlScroll}
            contentContainerStyle={styles.yamlContent}
          >
            <Text style={styles.yamlText} selectable>
              {selectedPipeline?.yaml_content || t("pipeline_no_yaml")}
            </Text>
          </ScrollView>
        </View>
      </Modal>
    </View>
  );
}

const makeStyles = (colors: AppColors) =>
  StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  list: { padding: spacing.lg },
  empty: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyBox: { alignItems: "center", gap: spacing.md },
  emptyText: { fontSize: fontSize.base, color: colors.textTertiary },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.lg,
    marginBottom: spacing.md,
  },
  cardTop: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
  },
  cardInfo: { flex: 1, gap: 4 },
  cardName: {
    fontSize: fontSize.base,
    fontWeight: "700",
    color: colors.textPrimary,
  },
  badgeRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  badge: {
    backgroundColor: colors.elevated,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  badgeText: {
    fontSize: fontSize.xs,
    fontWeight: "600",
    color: colors.textSecondary,
  },
  dateText: { fontSize: fontSize.xs, color: colors.textDisabled },
  // Modal
  modalContainer: { flex: 1, backgroundColor: colors.bg },
  modalHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.lg,
    backgroundColor: colors.surface,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  modalCancel: {
    fontSize: fontSize.base,
    color: colors.textSecondary,
  },
  modalTitle: {
    fontSize: fontSize.lg,
    fontWeight: "700",
    color: colors.textPrimary,
    flex: 1,
    textAlign: "center",
  },
  yamlScroll: { flex: 1 },
  yamlContent: { padding: spacing.lg },
  yamlText: {
    fontSize: fontSize.sm,
    fontFamily: "monospace",
    color: colors.textSecondary,
    lineHeight: 20,
  },
});
