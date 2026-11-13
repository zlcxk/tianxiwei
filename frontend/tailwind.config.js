/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        canvas: '#FAF7F2',
        card: '#FFFDF9',
        accent: '#8F4C56',
        'accent-soft': '#B06B74',
        'accent-deep': '#8F4C56',
        blush: '#F2DFE1',
        ink: '#2E292A',
        muted: '#6E6460',
        line: '#E4DCD3',
        moss: '#6F7A67',
        sand: '#D8C3A5',
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'sans-serif'],
        display: ['Georgia', 'Times New Roman', 'Songti SC', 'STSong', 'Noto Serif SC', 'serif'],
      },
      letterSpacing: {
        widest2: '0.22em',
      },
      boxShadow: {
        soft: '0 20px 60px -30px rgba(48, 43, 44, 0.22)',
        lift: '0 12px 30px -20px rgba(48, 43, 44, 0.3)',
      },
      animation: {
        'fade-up': 'fadeUp 0.5s ease both',
        'fade-in': 'fadeIn 0.35s ease both',
      },
      keyframes: {
        fadeUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
      },
    },
  },
  plugins: [],
}
