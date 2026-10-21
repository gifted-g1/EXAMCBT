/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      fontFamily: { sans: ['Inter', 'ui-sans-serif', 'system-ui', 'Segoe UI', 'Roboto', 'sans-serif'] },
      colors: {
        brand: { 50: '#eef2ff', 100: '#e0e7ff', 300: '#a5b4fc', 400: '#818cf8', 500: '#6366f1', 600: '#4f46e5', 700: '#4338ca', 900: '#1e1b4b' },
      },
      keyframes: {
        'fade-up': { '0%': { opacity: '0', transform: 'translateY(24px)' }, '100%': { opacity: '1', transform: 'translateY(0)' } },
        float: { '0%,100%': { transform: 'translateY(0)' }, '50%': { transform: 'translateY(-12px)' } },
        blob: {
          '0%,100%': { transform: 'translate(0,0) scale(1)' },
          '33%': { transform: 'translate(40px,-30px) scale(1.1)' },
          '66%': { transform: 'translate(-30px,20px) scale(0.95)' },
        },
        scan: { '0%': { transform: 'translateY(-10%)' }, '100%': { transform: 'translateY(110%)' } },
        'pulse-ring': { '0%': { transform: 'scale(0.8)', opacity: '0.7' }, '100%': { transform: 'scale(1.8)', opacity: '0' } },
        'gradient-x': { '0%,100%': { backgroundPosition: '0% 50%' }, '50%': { backgroundPosition: '100% 50%' } },
        'dash-flow': { to: { strokeDashoffset: '-24' } },
        'slide-in': { '0%': { opacity: '0', transform: 'translateX(-16px)' }, '100%': { opacity: '1', transform: 'translateX(0)' } },
      },
      animation: {
        'fade-up': 'fade-up 0.7s cubic-bezier(.2,.7,.2,1) both',
        float: 'float 6s ease-in-out infinite',
        blob: 'blob 18s ease-in-out infinite',
        scan: 'scan 2.8s ease-in-out infinite alternate',
        'pulse-ring': 'pulse-ring 2.4s cubic-bezier(.2,.6,.4,1) infinite',
        'gradient-x': 'gradient-x 8s ease infinite',
        'dash-flow': 'dash-flow 1s linear infinite',
        'slide-in': 'slide-in 0.5s ease-out both',
      },
    },
  },
  plugins: [],
}
