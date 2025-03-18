/* eslint-env node */
module.exports = {
  plugins: ["react", "@typescript-eslint"],
  extends: ["../.eslintrc.js", "next", "next/core-web-vitals", "plugin:@typescript-eslint/recommended"],
  rules: {
    "@typescript-eslint/no-unused-vars": "warn",
    "@typescript-eslint/no-explicit-any": "warn",
    "@typescript-eslint/no-duplicate-imports": "off",
    "@typescript-eslint/no-var-requires": "off",
    "react/react-in-jsx-scope": "off",
  },
};
