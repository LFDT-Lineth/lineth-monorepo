module.exports = {
  extends: "../../.eslintrc.js",
  env: {
    commonjs: true,
    es2021: true,
    node: true,
    jest: true,
  },
  parserOptions: {
    sourceType: "module",
  },
  rules: {
    "prettier/prettier": "error",
  },
};
