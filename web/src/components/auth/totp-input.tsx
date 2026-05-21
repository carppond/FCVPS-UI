import * as React from "react";
import { cn } from "@/lib/cn";

const CODE_LENGTH = 6;
const SHAKE_DURATION_MS = 200;

export interface TotpInputProps {
  /** Current value (parent-controlled). Always exactly CODE_LENGTH chars when complete. */
  value: string;
  /** Called on every change with the up-to-CODE_LENGTH-digit string. */
  onChange: (value: string) => void;
  /** Fired exactly once when the user enters the final digit. */
  onComplete?: (value: string) => void;
  /** When true, all cells render in error state and shake briefly. */
  hasError?: boolean;
  /** Disables all cells (e.g. while a verify request is in-flight). */
  disabled?: boolean;
  /** Optional aria-label for the group container. */
  "aria-label"?: string;
}

/**
 * Six-cell OTP input.
 *
 * Behavior:
 *   - Each cell accepts exactly one digit; entering a digit advances focus.
 *   - Backspace on an empty cell moves focus back one cell.
 *   - Arrow keys move focus left/right between cells.
 *   - Pasting a 6-digit string fills all cells and fires onComplete.
 *   - hasError triggers a 200ms shake animation and styles all cells red.
 */
export function TotpInput({
  value,
  onChange,
  onComplete,
  hasError = false,
  disabled = false,
  "aria-label": ariaLabel,
}: TotpInputProps) {
  const inputsRef = React.useRef<Array<HTMLInputElement | null>>([]);
  const [isShaking, setIsShaking] = React.useState(false);
  const completedRef = React.useRef(false);

  // Reset completion latch whenever the value drops below full length so the
  // onComplete callback fires once per "fully-typed" event, not on every char.
  React.useEffect(() => {
    if (value.length < CODE_LENGTH) completedRef.current = false;
  }, [value]);

  // Trigger shake animation each time hasError flips to true.
  React.useEffect(() => {
    if (!hasError) {
      setIsShaking(false);
      return;
    }
    setIsShaking(true);
    const timer = window.setTimeout(() => setIsShaking(false), SHAKE_DURATION_MS);
    return () => window.clearTimeout(timer);
  }, [hasError]);

  const cells = React.useMemo<string[]>(() => {
    const padded = value.padEnd(CODE_LENGTH, " ").slice(0, CODE_LENGTH);
    return Array.from(padded).map((ch) => (ch === " " ? "" : ch));
  }, [value]);

  const commit = (next: string) => {
    const sliced = next.slice(0, CODE_LENGTH);
    onChange(sliced);
    if (sliced.length === CODE_LENGTH && !completedRef.current) {
      completedRef.current = true;
      onComplete?.(sliced);
    }
  };

  const focusCell = (index: number) => {
    const clamped = Math.max(0, Math.min(CODE_LENGTH - 1, index));
    inputsRef.current[clamped]?.focus();
    inputsRef.current[clamped]?.select();
  };

  const handleChange = (index: number, raw: string) => {
    // Only take the last typed digit (handles browsers that report full string).
    const digit = raw.replace(/\D/g, "").slice(-1);
    if (!digit) return;

    const chars = value.padEnd(CODE_LENGTH, " ").split("");
    chars[index] = digit;
    // Trim trailing spaces so the committed value is the minimal prefix.
    const trimmed = chars.join("").replace(/ +$/, "");
    commit(trimmed);
    if (index < CODE_LENGTH - 1) focusCell(index + 1);
  };

  const handleKeyDown = (
    index: number,
    event: React.KeyboardEvent<HTMLInputElement>,
  ) => {
    if (event.key === "Backspace") {
      if (cells[index]) {
        const chars = cells.slice();
        chars[index] = "";
        commit(chars.join("").replace(/ +$/, ""));
      } else if (index > 0) {
        // Empty cell + Backspace → clear previous cell + move focus back.
        const chars = cells.slice();
        chars[index - 1] = "";
        commit(chars.join("").replace(/ +$/, ""));
        focusCell(index - 1);
      }
      event.preventDefault();
      return;
    }
    if (event.key === "ArrowLeft") {
      focusCell(index - 1);
      event.preventDefault();
      return;
    }
    if (event.key === "ArrowRight") {
      focusCell(index + 1);
      event.preventDefault();
    }
  };

  const handlePaste = (
    index: number,
    event: React.ClipboardEvent<HTMLInputElement>,
  ) => {
    const text = event.clipboardData.getData("text").replace(/\D/g, "");
    if (!text) return;
    event.preventDefault();

    const chars = cells.slice();
    for (let i = 0; i < text.length && index + i < CODE_LENGTH; i += 1) {
      chars[index + i] = text[i];
    }
    commit(chars.join("").replace(/ +$/, ""));
    const nextIndex = Math.min(index + text.length, CODE_LENGTH - 1);
    focusCell(nextIndex);
  };

  return (
    <div
      role="group"
      aria-label={ariaLabel}
      data-testid="totp-input"
      data-shaking={isShaking ? "true" : "false"}
      className={cn(
        "flex items-center justify-center gap-2",
        isShaking && "animate-totp-shake",
      )}
    >
      {cells.map((char, index) => (
        <input
          key={index}
          ref={(el) => {
            inputsRef.current[index] = el;
          }}
          type="text"
          inputMode="numeric"
          autoComplete={index === 0 ? "one-time-code" : "off"}
          maxLength={1}
          value={char}
          disabled={disabled}
          aria-label={`Digit ${index + 1}`}
          data-testid={`totp-cell-${index}`}
          onChange={(e) => handleChange(index, e.target.value)}
          onKeyDown={(e) => handleKeyDown(index, e)}
          onPaste={(e) => handlePaste(index, e)}
          onFocus={(e) => e.currentTarget.select()}
          className={cn(
            "h-14 w-12 rounded-[var(--radius-md)] border bg-[var(--color-surface)]",
            "text-center font-mono text-[var(--font-size-2xl)] font-semibold",
            "text-[var(--color-text-primary)]",
            "transition-colors duration-[var(--duration-fast)]",
            "focus:outline-none focus:ring-2 focus:ring-offset-0",
            "disabled:cursor-not-allowed disabled:opacity-50",
            hasError
              ? "border-[var(--color-error)] text-[var(--color-error)] focus:ring-[var(--color-error)]"
              : "border-[var(--color-border-strong)] focus:ring-[var(--color-primary)] focus:border-[var(--color-primary)]",
          )}
        />
      ))}
    </div>
  );
}

export const TOTP_CODE_LENGTH = CODE_LENGTH;
