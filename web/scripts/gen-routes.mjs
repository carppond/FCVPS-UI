#!/usr/bin/env node
// Standalone route-tree generation script.
// Mirrors what @tanstack/router-plugin runs in dev/build mode so we can
// regenerate routeTree.gen.ts from CI without booting Vite.
// Resolve via the pnpm-managed path since `@tanstack/router-generator` is a
// transitive dep of router-plugin and not hoisted to the top-level resolver.
const genPath =
  "../node_modules/.pnpm/@tanstack+router-generator@1.167.6/node_modules/@tanstack/router-generator/dist/esm/index.js";
const { Generator, getConfig } = await import(genPath);
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");

const config = await getConfig(
  { routesDirectory: "./src/routes" },
  root,
);

const generator = new Generator({ config, root });
await generator.run();
console.log("routeTree.gen.ts regenerated.");
