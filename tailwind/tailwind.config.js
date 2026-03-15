/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    '../templates/**/*.html',
    '../static/**/*.js',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ink:       '#1a1a18',
        'ink-soft':'#4a4a45',
        'ink-muted':'#8a8a82',
        paper:     '#faf9f6',
        'paper-alt':'#f2f0eb',
        rule:      '#e0ddd6',
        accent:    '#c8402a',
        'accent-alt':'#2a6e5e',
      },
      fontFamily: {
        display: ['"Playfair Display"', 'Georgia', 'serif'],
        body:    ['"Source Serif 4"', 'Georgia', 'serif'],
        mono:    ['"JetBrains Mono"', 'monospace'],
      },
      fontSize: {
        '2xs': '0.65rem',
        '3xs': '0.6rem',
      },
      maxWidth: {
        reading: '680px',
        content: '860px',
      },
      typography: ({ theme }) => ({
        // Wires the prose plugin into our custom ink/paper color tokens
        neutral: {
          css: {
            '--tw-prose-body':        theme('colors.ink'),
            '--tw-prose-headings':    theme('colors.ink'),
            '--tw-prose-links':       theme('colors.accent'),
            '--tw-prose-bold':        theme('colors.ink'),
            '--tw-prose-code':        theme('colors.ink'),
            '--tw-prose-quotes':      theme('colors.ink-soft'),
            '--tw-prose-quote-borders': theme('colors.rule'),
            '--tw-prose-hr':          theme('colors.rule'),
          },
        },
      }),
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
};