package traffic

// Canonical system_settings keys owned by M-TRAFFIC. Centralised here so the
// aggregator, monthly reset, threshold checker and HTTP handler all reference
// the same literal — typos surface at compile time instead of as silent
// no-ops at runtime.
const (
	// SettingMonthlyTrafficLimit holds the user-wide monthly limit in BYTES.
	// Empty / 0 means "no limit"; the threshold checker is then a no-op.
	SettingMonthlyTrafficLimit = "monthly_traffic_limit"

	// SettingMonthlyResetDay is the day-of-month (1-28) on which the billing
	// cycle resets. Values outside the valid range fall back to
	// DefaultMonthlyResetDay.
	SettingMonthlyResetDay = "monthly_reset_day"

	// SettingTrafficThresholdPercents is a comma-separated list of the
	// percentage levels (out of 100) at which the threshold checker should
	// notify. Defaults to "80,90,100". A user may opt out of a particular
	// level by removing it from the list.
	SettingTrafficThresholdPercents = "traffic_threshold_percents"

	// settingTrafficLastThresholdPrefix is the per-user state key recording
	// which threshold percentages have already fired for the current month.
	// Format: "<userID>:<YYYY-MM>" maps to a value like "80,90".
	settingTrafficLastThresholdPrefix = "traffic_threshold_state:"

	// settingTrafficLastResetPrefix is the per-user state key recording the
	// YYYY-MM that has been "reset" — used to make MonthlyReset idempotent so
	// rerunning on the same day does not double-fire the notification.
	settingTrafficLastResetPrefix = "traffic_last_reset:"
)

// DefaultMonthlyResetDay matches PRD M-TRAFFIC default.
const DefaultMonthlyResetDay = 1

// DefaultThresholdPercents matches PRD M-TRAFFIC default.
var DefaultThresholdPercents = []int{80, 90, 100}
