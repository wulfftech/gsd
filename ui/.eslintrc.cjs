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
  settings: {
    react: { version: '18.2' },
    tailwindcss: {
      // Project CSS classes, not Tailwind utilities. Listed here so the rule
      // keeps catching genuine Tailwind typos instead of being switched off.
      // `quill-root`/`quill-variant-*` are styled in RichTextEditor.css; the
      // rest are semantic hooks carried on elements that Joy UI styles via sx.
      whitelist: [
        'logo',
        'calendar-dual',
        'feature-icon',
        'feature-title',
        'option-icon',
        'option-title',
        'selected',
        'quill-root',
        'quill-variant-.*',
      ],
    },
  },
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
    {
      // react-refresh/only-export-components guards Fast Refresh boundaries,
      // which only apply to component modules. These are provider modules
      // (where co-locating a context with its consumer hook is the standard
      // React pattern) and plain utility modules that happen to be .jsx and
      // export PascalCase functions the rule mistakes for components.
      // Splitting them would churn imports across the app for no runtime gain.
      files: [
        'src/contexts/**',
        'src/service/**',
        'src/hooks/useAuth.jsx',
        'src/utils/Chores.jsx',
        'src/utils/Fetcher.jsx',
      ],
      rules: { 'react-refresh/only-export-components': 'off' },
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
