import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig({ plugins: [vue()], resolve: { alias: { '@': '/src' } }, server: { port: 5173, proxy: { '/api': { target: 'http://localhost:8819', changeOrigin: true, rewrite: p => p.replace(/^\/api/, '') } } } })
