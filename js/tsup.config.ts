import { defineConfig } from "tsup";

export default defineConfig({
  entry: { index: "src/index.ts", webhooks: "src/webhooks.ts" },
  format: ["esm", "cjs"],
  dts: true,
  sourcemap: true,
  clean: true,
  // Share one TokTokDataError class between the main entry and `webhooks` in ESM and CJS.
  splitting: true,
  target: "es2022",
  platform: "neutral",
  external: ["node:crypto", "node:buffer"],
});
