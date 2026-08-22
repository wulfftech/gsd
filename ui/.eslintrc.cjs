module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    'eslint:recommended',
    'plugin:react/recommended',
    'plugin:react/jsx-runtime',
    'plugin:react-hooks/recommended',
    'plugin:prettier/recommended',
    'plugin:tailwindcss/recommended',
  ],
  ignorePatterns: [
    'dist',
    '.eslintrc.cjs',
    'tailwind.config.js',
    'postcss.config.js',
  ],
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
  settings: { react: { version: '18.2' } },
  plugins: [
    'react-refresh',
    'simple-import-sort',
    'sort-destructure-keys',
    'sort-keys-fix',
    'prettier',

    'tailwindcss',
  ],
  overrides: [
    {
      // Build and tooling scripts run under Node, not in the browser.
      files: ['bump-version.js', 'vite.config.js'],
      env: { node: true, browser: false },
    },
  ],
  rules: {
    'react-refresh/only-export-components': [
      'warn',
      { allowConstantExport: true },
    ],
    'react/prop-types': 'off',
    // Deliberate discards are prefixed with an underscore, a convention the
    // code already uses. It is load-bearing in BottomSheetModal/FadeModal,
    // where the named bindings exist purely to keep those props out of the
    // ...modalProps rest spread -- deleting them would change what reaches
    // the DOM. ignoreRestSiblings covers that idiom directly.
    'no-unused-vars': [
      'error',
      {
        args: 'after-used',
        argsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_',
        destructuredArrayIgnorePattern: '^_',
        ignoreRestSiblings: true,
        varsIgnorePattern: '^_',
      },
    ],
  },
}
