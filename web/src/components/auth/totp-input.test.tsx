import * as React from "react";
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { TotpInput, TOTP_CODE_LENGTH } from "./totp-input";

/**
 * Small harness wrapping TotpInput so each test can read the latest
 * controlled value via a data attribute on the wrapping element.
 */
function Harness({
  onComplete,
  hasError = false,
}: {
  onComplete?: (v: string) => void;
  hasError?: boolean;
}) {
  const [value, setValue] = React.useState("");
  return (
    <div data-testid="harness" data-value={value}>
      <TotpInput
        value={value}
        onChange={setValue}
        onComplete={onComplete}
        hasError={hasError}
      />
    </div>
  );
}

const cellOf = (i: number) =>
  screen.getByTestId(`totp-cell-${i}`) as HTMLInputElement;

function typeDigit(cell: HTMLInputElement, digit: string) {
  fireEvent.change(cell, { target: { value: digit } });
}

describe("TotpInput", () => {
  it("auto-advances to the next cell after typing a digit", () => {
    render(<Harness />);
    typeDigit(cellOf(0), "1");
    expect(cellOf(0).value).toBe("1");
    expect(document.activeElement).toBe(cellOf(1));
  });

  it("pastes a 6-digit string and fills every cell", () => {
    render(<Harness />);
    const first = cellOf(0);
    first.focus();

    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (kind: string) => (kind === "text" ? "123456" : ""),
      },
    });
    fireEvent(first, pasteEvent);

    expect(cellOf(0).value).toBe("1");
    expect(cellOf(5).value).toBe("6");
    expect(screen.getByTestId("harness").dataset.value).toBe("123456");
  });

  it("invokes onComplete exactly once when all 6 digits are typed", () => {
    const onComplete = vi.fn();
    render(<Harness onComplete={onComplete} />);
    for (let i = 0; i < TOTP_CODE_LENGTH; i += 1) {
      typeDigit(cellOf(i), String(i + 1));
    }
    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onComplete).toHaveBeenCalledWith("123456");
  });

  it("invokes onComplete when a 6-digit string is pasted", () => {
    const onComplete = vi.fn();
    render(<Harness onComplete={onComplete} />);
    const first = cellOf(0);
    first.focus();
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: { getData: () => "987654" },
    });
    fireEvent(first, pasteEvent);
    expect(onComplete).toHaveBeenCalledWith("987654");
  });

  it("renders error styling and exposes shake state when hasError flips to true", () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(<Harness hasError={false} />);
      const group = screen.getByTestId("totp-input");
      expect(group.dataset.shaking).toBe("false");

      rerender(<Harness hasError={true} />);
      expect(group.dataset.shaking).toBe("true");

      // Shake auto-resets after 200ms.
      act(() => {
        vi.advanceTimersByTime(220);
      });
      expect(group.dataset.shaking).toBe("false");
    } finally {
      vi.useRealTimers();
    }
  });

  it("supports backspace to clear and step backwards on an empty cell", () => {
    render(<Harness />);
    typeDigit(cellOf(0), "1");
    typeDigit(cellOf(1), "2");
    // Focus is now on cell 2 (after auto-advance). Backspace on the empty
    // cell 2 should clear cell 1 and move focus back to cell 1.
    fireEvent.keyDown(cellOf(2), { key: "Backspace" });
    expect(cellOf(1).value).toBe("");
    expect(document.activeElement).toBe(cellOf(1));
  });

  it("exposes the correct number of cells", () => {
    render(<Harness />);
    for (let i = 0; i < TOTP_CODE_LENGTH; i += 1) {
      const cell = screen.queryByTestId(`totp-cell-${i}`);
      expect(cell).not.toBeNull();
    }
  });

  it("arrow keys move focus between cells", () => {
    render(<Harness />);
    cellOf(2).focus();
    fireEvent.keyDown(cellOf(2), { key: "ArrowLeft" });
    expect(document.activeElement).toBe(cellOf(1));
    fireEvent.keyDown(cellOf(1), { key: "ArrowRight" });
    expect(document.activeElement).toBe(cellOf(2));
  });
});
