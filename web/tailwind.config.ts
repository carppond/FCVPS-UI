import type { Config } from "tailwindcss";

/**
 * Tailwind v4 uses CSS-first configuration.
 * Design tokens are declared in src/styles/globals.css via @theme blocks.
 * This file only sets the content paths and any JS-level plugin config.
 */
const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {},
  },
  plugins: [],
};

export default config;
