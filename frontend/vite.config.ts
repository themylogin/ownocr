import path from 'node:path'
import process from 'node:process'
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import sirv from 'sirv'

const proxyTarget = process.env.VITE_PROXY_TARGET ?? 'http://localhost:8080'
const dataDir = process.env.DATA_DIR ?? '/data'

// Production serves /files from nginx. The dev server has no nginx, so it
// serves the same directory here. The Go backend never serves images.
function servePageFiles(): Plugin {
  return {
    name: 'ownocr-page-files',
    configureServer(server) {
      const pages = path.join(dataDir, 'pages')
      const serve = sirv(pages, { dev: true, etag: true, dotfiles: false })

      // sirv falls through to next() only when it is given one. Withholding it
      // makes a miss a 404 instead of Vite's index.html SPA fallback.
      server.middlewares.use('/files', (req, res) => serve(req, res))
    },
  }
}

export default defineConfig({
  plugins: [vue(), servePageFiles()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    // Vite blocks unknown Host headers. The e2e browser reaches the dev server
    // by its compose service name, not by localhost.
    allowedHosts: ['localhost', '127.0.0.1', 'frontend'],
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
