import js from "@eslint/js";
import tseslint from "typescript-eslint";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";
import jsxA11y from "eslint-plugin-jsx-a11y";
import prettier from "eslint-config-prettier";

// React 19 + TypeScript flat config. Scoped to src/ and tests/.
export default tseslint.config(
  {
    ignores: [
      "dist/**",
      "build/**",
      ".react-router/**",
      "node_modules/**",
      "coverage/**",
      "src/portal/api/__generated__/**",
    ],
  },
  js.configs.recommended,
  // Type-checked rules apply to TS files only.
  {
    files: ["**/*.{ts,tsx,mts,cts}"],
    extends: [...tseslint.configs.strictTypeChecked, ...tseslint.configs.stylisticTypeChecked],
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      "@typescript-eslint/consistent-type-imports": [
        "error",
        { prefer: "type-imports", fixStyle: "inline-type-imports" },
      ],
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
  // Plain JS/MJS/CJS files (eslint.config.js itself, etc.) — no type-checking.
  {
    files: ["**/*.{js,mjs,cjs}"],
    ...tseslint.configs.disableTypeChecked,
  },
  // React rules for .tsx / .jsx.
  {
    files: ["**/*.{tsx,jsx}"],
    plugins: {
      react,
      "react-hooks": reactHooks,
      "jsx-a11y": jsxA11y,
    },
    rules: {
      ...react.configs.flat.recommended.rules,
      ...react.configs.flat["jsx-runtime"].rules,
      ...reactHooks.configs.recommended.rules,
      ...jsxA11y.flatConfigs.recommended.rules,
    },
    settings: {
      react: { version: "detect" },
    },
  },
  prettier,
);
