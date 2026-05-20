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
import { router } from "./App";
import "@/styles/globals.css";

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
