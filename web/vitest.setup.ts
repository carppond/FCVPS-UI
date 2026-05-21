import "@testing-library/jest-dom/vitest";

// jsdom lacks ResizeObserver + scrollIntoView, which cmdk + Radix popovers
// rely on at mount. No-op polyfills keep the dialog from crashing.
if (typeof globalThis.ResizeObserver === "undefined") {
  class StubResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  // @ts-expect-error — assigning a polyfill on the global is the whole point.
  globalThis.ResizeObserver = StubResizeObserver;
}
if (typeof Element !== "undefined" && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function () {
    /* no-op */
  };
}
