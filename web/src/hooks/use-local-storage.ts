import { useCallback, useState } from "react";
import { safeGet, safeSet, safeRemove } from "@/lib/storage";

/**
 * Typed localStorage hook with JSON serialization.
 * Falls back to `initialValue` if the key is missing or localStorage is unavailable.
 *
 * @returns [storedValue, setValue, removeValue]
 */
export function useLocalStorage<T>(
  key: string,
  initialValue: T,
): [T, (value: T | ((prev: T) => T)) => void, () => void] {
  const [storedValue, setStoredValue] = useState<T>(() => {
    const raw = safeGet(key);
    if (raw === null) return initialValue;
    try {
      return JSON.parse(raw) as T;
    } catch {
      return initialValue;
    }
  });

  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      setStoredValue((prev) => {
        const nextValue = typeof value === "function" ? (value as (prev: T) => T)(prev) : value;
        safeSet(key, JSON.stringify(nextValue));
        return nextValue;
      });
    },
    [key],
  );

  const removeValue = useCallback(() => {
    safeRemove(key);
    setStoredValue(initialValue);
  }, [key, initialValue]);

  return [storedValue, setValue, removeValue];
}
