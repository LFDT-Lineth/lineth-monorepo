import baseConfig from "../prettier.config.mjs";

/** @type {import("prettier").Config} */
export default {
  ...baseConfig,
  plugins: ["prettier-plugin-solidity"],
  overrides: [
    {
      files: "*.sol",
      options: {
        parser: "slang",
        printWidth: 120,
        tabWidth: 2,
        useTabs: false,
        singleQuote: false,
        bracketSpacing: true,
      },
    },
  ],
};
