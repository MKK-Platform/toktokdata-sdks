import { defineConfig } from "tsup";

export default defineConfig({
  entry: { index: "src/index.ts", bin: "src/bin.ts" },
  format: ["esm"],
  dts: { entry: { index: "src/index.ts" } },
  sourcemap: true,
  clean: true,
  target: "node18",
  platform: "node",
  // Needed by `bin.js` (npx runs it directly); harmless at the top of `index.js`.
  banner: { js: "#!/usr/bin/env node" },
});
