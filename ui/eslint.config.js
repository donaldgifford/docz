import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import jsxA11y from "eslint-plugin-jsx-a11y";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "dist",
      "dist-msw",
      "dist-server",
      "dist-review",
      "coverage",
      "node_modules",
      "test-results",
      "playwright-report",
      "src/api/__generated__",
      "src/cli/__generated__",
      "review/__generated__",
      "review/compiled.ts",
      "dist-review-bin",
      "public/mockServiceWorker.js",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.strictTypeChecked,
  ...tseslint.configs.stylisticTypeChecked,
  reactHooks.configs.flat.recommended,
  jsxA11y.flatConfigs.recommended,
  {
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
  {
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["mermaid/*"],
              message:
                "Import the bare `mermaid` specifier. Its package exports map to dist/mermaid.core.mjs; the minified sibling in dist/ contains syntax es-module-lexer (which Vite uses) rejects, so deep-pathing breaks the build.",
            },
          ],
        },
      ],
    },
  },
  {
    // Plain JS at the repo root (this file) has no tsconfig project.
    files: ["**/*.js"],
    ...tseslint.configs.disableTypeChecked,
  },
  prettier,
);
