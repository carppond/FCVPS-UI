import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider } from "@tanstack/react-router";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { I18nextProvider } from "react-i18next";

// Initialize i18n before rendering.
import i18n from "@/lib/i18n";
// Apply persisted theme on page load.
import { applyTheme, getCurrentTheme } from "@/lib/theme";
// Shared QueryClient singleton (T-7): defaults, retry policy, global error handler.
import { queryClient } from "@/lib/query-client";
import { extractSilentPrefixFromURL } from "@/lib/silent-prefix";
import { router } from "./App";
import "@/styles/globals.css";

// Capture silent-mode prefix from the entry URL BEFORE the router boots.
// The hub enforces /_app/<32hex>/<route> for every non-whitelisted request;
// we cache the prefix in localStorage so every later apiFetch() picks it up
// transparently via prefixedPath(). We also rewrite the browser URL to the
// canonical /route path so TanStack Router doesn't try to match the prefixed
// form. This MUST run before createRoot() to avoid the router racing an
// initial navigation against the unprefixed routes.
extractSilentPrefixFromURL();

// Apply the stored theme before paint to avoid flash.
applyTheme(getCurrentTheme());

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Root element #root not found");

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        {import.meta.env.DEV && (
          <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-left" />
        )}
      </QueryClientProvider>
    </I18nextProvider>
  </React.StrictMode>,
);
