import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    host: true,
    proxy: {
      '/api': 'http://localhost:5170',
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          // Split vendor chunks for better caching
          if (id.includes('node_modules')) {
            // React core
            if (id.includes('react') || id.includes('react-dom')) {
              return 'react-vendor';
            }
            // Radix UI components
            if (id.includes('@radix-ui')) {
              return 'ui-vendor';
            }
            // Map libraries
            if (id.includes('leaflet')) {
              return 'map-vendor';
            }
            // Markdown processing
            if (id.includes('marked') || id.includes('react-markdown') || id.includes('remark-gfm')) {
              return 'markdown-vendor';
            }
            // Utility libraries
            if (id.includes('class-variance-authority') || id.includes('clsx') || id.includes('lucide-react') || id.includes('tailwind-merge') || id.includes('sonner')) {
              return 'utils-vendor';
            }
          }
        },
      },
    },
    chunkSizeWarningLimit: 1000, // Increase limit to 1000 kB to reduce noise
  },
})
