/**
 * localStorage wrapper with availability detection.
 * Falls back to null / false on private mode or quota errors — never throws.
 */

let _isAvailable: boolean | undefined;

/** Detect localStorage availability once and cache the result. */
function isLocalStorageAvailable(): boolean {
  if (_isAvailable !== undefined) return _isAvailable;
  try {
    const probe = "__sgvps_probe__";
    localStorage.setItem(probe, "1");
    localStorage.removeItem(probe);
    _isAvailable = true;
  } catch {
    _isAvailable = false;
  }
  return _isAvailable;
}

/**
 * Safely get a value from localStorage.
 * Returns null if unavailable or the key does not exist.
 */
export function safeGet(key: string): string | null {
  if (!isLocalStorageAvailable()) return null;
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

/**
 * Safely set a value in localStorage.
 * Returns false if unavailable or a quota error occurs.
 */
export function safeSet(key: string, value: string): boolean {
  if (!isLocalStorageAvailable()) return false;
  try {
    localStorage.setItem(key, value);
    return true;
  } catch {
    return false;
  }
}

/**
 * Safely remove a key from localStorage.
 * Returns false if unavailable.
 */
export function safeRemove(key: string): boolean {
  if (!isLocalStorageAvailable()) return false;
  try {
    localStorage.removeItem(key);
    return true;
  } catch {
    return false;
  }
}
