import * as React from "react";
import * as Popover from "@radix-ui/react-popover";
import {
  format,
  startOfMonth,
  endOfMonth,
  eachDayOfInterval,
  getDay,
  addMonths,
  subMonths,
  addDays,
  isSameDay,
  isToday,
  differenceInCalendarDays,
} from "date-fns";
import { zhCN, enUS, ja, ko } from "date-fns/locale";
import { ChevronLeft, ChevronRight, Calendar } from "lucide-react";
import { cn } from "@/lib/cn";
import i18n from "@/lib/i18n";

const LOCALE_MAP: Record<string, Locale> = {
  "zh-CN": zhCN,
  en: enUS,
  ja,
  ko,
};

function getLocale(): Locale {
  return LOCALE_MAP[i18n.language] ?? enUS;
}

const MONTHS_ZH = ["1月","2月","3月","4月","5月","6月","7月","8月","9月","10月","11月","12月"];
const MONTHS_EN = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];

function monthOptions(locale: Locale): string[] {
  return locale === zhCN || locale === ja || locale === ko ? MONTHS_ZH : MONTHS_EN;
}

interface DatePickerProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

export function DatePicker({
  value,
  onChange,
  placeholder,
  className,
}: DatePickerProps) {
  const [open, setOpen] = React.useState(false);
  const selected = value ? new Date(value + "T00:00:00") : null;
  const [viewMonth, setViewMonth] = React.useState(
    () => selected ?? new Date(),
  );

  React.useEffect(() => {
    if (open && selected) setViewMonth(selected);
  }, [open]);

  const locale = getLocale();
  const monthStart = startOfMonth(viewMonth);
  const monthEnd = endOfMonth(viewMonth);
  const days = eachDayOfInterval({ start: monthStart, end: monthEnd });
  const startDow = getDay(monthStart);
  const prevMonthEnd = endOfMonth(subMonths(viewMonth, 1));
  const prevPadDays = Array.from({ length: startDow }, (_, i) => {
    const d = new Date(prevMonthEnd);
    d.setDate(d.getDate() - (startDow - 1 - i));
    return d;
  });

  const daysRemaining = selected
    ? differenceInCalendarDays(selected, new Date())
    : null;

  const selectDay = (d: Date) => {
    onChange(format(d, "yyyy-MM-dd"));
    setOpen(false);
  };

  const viewYear = viewMonth.getFullYear();
  const viewMon = viewMonth.getMonth();
  const years = Array.from({ length: 11 }, (_, i) => new Date().getFullYear() - 2 + i);

  const weekDays = React.useMemo(() => {
    const base = new Date(2024, 0, 7);
    return Array.from({ length: 7 }, (_, i) => {
      const d = new Date(base);
      d.setDate(d.getDate() + i);
      return format(d, "EEEEEE", { locale }).toUpperCase();
    });
  }, [locale]);

  const quickAdd = (n: number) => {
    const base = selected ?? new Date();
    onChange(format(addDays(base, n), "yyyy-MM-dd"));
    setOpen(false);
  };

  const clear = () => {
    onChange("");
    setOpen(false);
  };

  const defaultPlaceholder = locale === zhCN ? "选择日期…" : "Select date…";

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          className={cn(
            "flex h-11 w-full items-center gap-2.5 rounded-xl",
            "border border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)]",
            "px-3.5 text-left text-sm transition-all duration-150",
            "hover:border-[var(--color-border-strong)]",
            "focus-visible:border-[var(--color-primary)] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-[var(--color-primary)]/15",
            !value && "text-[var(--color-text-disabled)]",
            value && "text-[var(--color-text-primary)]",
            className,
          )}
        >
          <Calendar className="h-4 w-4 shrink-0 text-[var(--color-primary)]" />
          <span className="flex-1 truncate font-medium tabular-nums">
            {selected ? format(selected, "yyyy-MM-dd") : (placeholder ?? defaultPlaceholder)}
          </span>
          {daysRemaining !== null && (
            <span
              className={cn(
                "shrink-0 rounded-md px-2 py-0.5 text-[9px] font-bold",
                daysRemaining > 7 && "bg-[rgba(52,211,153,.1)] text-[var(--color-success)]",
                daysRemaining > 0 && daysRemaining <= 7 && "bg-[rgba(251,191,36,.1)] text-[var(--color-warning)]",
                daysRemaining <= 0 && "bg-[rgba(248,113,113,.1)] text-[var(--color-error)]",
              )}
            >
              {daysRemaining > 0 ? `${daysRemaining}d` : daysRemaining === 0 ? "today" : `${Math.abs(daysRemaining)}d ago`}
            </span>
          )}
        </button>
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={6}
          className={cn(
            "z-[var(--z-popover,50)] w-[340px] overflow-hidden rounded-[20px]",
            "border border-[var(--color-border)] bg-[var(--glass-dialog,var(--color-surface))]",
            "shadow-[0_32px_80px_rgba(0,0,0,0.6),0_0_0_1px_rgba(255,255,255,0.04)]",
            "backdrop-blur-xl",
            "animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95",
          )}
        >
          {/* Top light edge */}
          <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/[.08] to-transparent" />

          {/* Header: year/month selects + arrows */}
          <div className="flex items-center justify-between px-5 pt-4 pb-2">
            <div className="flex items-center gap-1.5">
              <select
                value={viewYear}
                onChange={(e) => {
                  const d = new Date(viewMonth);
                  d.setFullYear(Number(e.target.value));
                  setViewMonth(d);
                }}
                className="appearance-none border-none bg-transparent text-sm font-bold text-[var(--color-text-primary)] outline-none cursor-pointer"
              >
                {years.map((y) => (
                  <option key={y} value={y} className="bg-[var(--color-surface)] text-[var(--color-text-primary)]">{y}</option>
                ))}
              </select>
              <span className="text-[13px] text-[var(--color-text-disabled)]">·</span>
              <select
                value={viewMon}
                onChange={(e) => {
                  const d = new Date(viewMonth);
                  d.setMonth(Number(e.target.value));
                  setViewMonth(d);
                }}
                className="appearance-none border-none bg-transparent text-sm font-bold text-[var(--color-text-primary)] outline-none cursor-pointer"
              >
                {monthOptions(locale).map((m, i) => (
                  <option key={i} value={i} className="bg-[var(--color-surface)] text-[var(--color-text-primary)]">{m}</option>
                ))}
              </select>
            </div>
            <div className="flex gap-0.5">
              <button
                type="button"
                onClick={() => setViewMonth(subMonths(viewMonth, 1))}
                className="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-[var(--color-border)] text-[var(--color-text-tertiary)] transition hover:bg-white/[.06] hover:text-[var(--color-text-primary)]"
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                onClick={() => setViewMonth(addMonths(viewMonth, 1))}
                className="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-[var(--color-border)] text-[var(--color-text-tertiary)] transition hover:bg-white/[.06] hover:text-[var(--color-text-primary)]"
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>

          {/* Weekday headers */}
          <div className="grid grid-cols-7 px-4 mb-0.5">
            {weekDays.map((wd) => (
              <span
                key={wd}
                className="py-1.5 text-center text-[10px] font-bold tracking-wide text-[var(--color-text-disabled)]"
              >
                {wd}
              </span>
            ))}
          </div>

          {/* Day grid */}
          <div className="grid grid-cols-7 gap-[3px] px-4 pb-2">
            {prevPadDays.map((d) => (
              <button
                key={`pad-${d.getTime()}`}
                type="button"
                onClick={() => selectDay(d)}
                className="flex h-[38px] items-center justify-center rounded-[10px] text-[12px] text-[var(--color-text-disabled)] transition hover:bg-white/[.04]"
              >
                {d.getDate()}
              </button>
            ))}
            {days.map((d) => {
              const sel = selected && isSameDay(d, selected);
              const today = isToday(d);
              return (
                <button
                  key={d.toISOString()}
                  type="button"
                  onClick={() => selectDay(d)}
                  className={cn(
                    "flex h-[38px] items-center justify-center rounded-[10px] text-[13px] font-medium transition-all duration-100",
                    sel
                      ? "bg-[var(--color-primary)] text-white font-bold shadow-[0_3px_12px_rgba(255,99,99,0.35)]"
                      : "text-[var(--color-text-secondary)] hover:bg-white/[.05] hover:text-[var(--color-text-primary)]",
                    today && !sel && "bg-[rgba(255,99,99,.06)] text-[var(--color-primary)] font-bold",
                  )}
                >
                  {d.getDate()}
                </button>
              );
            })}
          </div>

          {/* Footer: quick buttons */}
          <div className="flex items-center justify-between border-t border-[var(--color-border)] px-5 py-3">
            <div className="flex gap-1.5">
              <QuickChip active onClick={() => selectDay(new Date())}>
                {locale === zhCN ? "今天" : "Today"}
              </QuickChip>
              <QuickChip onClick={() => quickAdd(30)}>+30d</QuickChip>
              <QuickChip onClick={() => quickAdd(90)}>+90d</QuickChip>
              <QuickChip onClick={() => quickAdd(365)}>+1y</QuickChip>
            </div>
            <QuickChip onClick={clear}>
              {locale === zhCN ? "清除" : "Clear"}
            </QuickChip>
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

function QuickChip({
  active,
  onClick,
  children,
}: {
  active?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-md px-2.5 py-1 text-[10px] font-semibold transition-all duration-100",
        active
          ? "border border-[rgba(255,99,99,.2)] bg-[rgba(255,99,99,.1)] text-[var(--color-primary)]"
          : "border border-[var(--color-border)] bg-transparent text-[var(--color-text-tertiary)] hover:bg-white/[.04] hover:text-[var(--color-text-primary)] hover:border-[var(--color-border-strong)]",
      )}
    >
      {children}
    </button>
  );
}

interface DateTimePickerProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

export function DateTimePicker({
  value,
  onChange,
  placeholder,
  className,
}: DateTimePickerProps) {
  const dateVal = value ? value.slice(0, 10) : "";
  const timeVal = value ? value.slice(11, 16) : "";

  const handleDateChange = (d: string) => {
    if (!d) {
      onChange("");
      return;
    }
    onChange(d + "T" + (timeVal || "00:00"));
  };

  const handleTimeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const t = e.target.value;
    if (!dateVal) {
      onChange(format(new Date(), "yyyy-MM-dd") + "T" + t);
    } else {
      onChange(dateVal + "T" + t);
    }
  };

  return (
    <div className={cn("flex gap-2", className)}>
      <div className="flex-1">
        <DatePicker value={dateVal} onChange={handleDateChange} placeholder={placeholder} />
      </div>
      <input
        type="time"
        value={timeVal}
        onChange={handleTimeChange}
        className={cn(
          "h-11 w-[100px] rounded-xl",
          "border border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)]",
          "px-3 text-center font-mono text-sm font-medium text-[var(--color-text-primary)]",
          "transition-all focus-visible:border-[var(--color-primary)] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-[var(--color-primary)]/15",
        )}
      />
    </div>
  );
}
