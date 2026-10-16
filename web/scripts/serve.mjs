#!/usr/bin/env node
// web/scripts/serve.mjs —— 零依赖本地静态服务
//
// 用途: 以 http 方式托管 web/ 静态站点（推荐方式；file:// 双击打开亦可工作，
//       但浏览器对 file:// 限制更多，日常开发建议走本服务）。
// 运行: npm start            （默认端口 8137）
//       node scripts/serve.mjs 9000          （指定端口）

import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { dirname, extname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PORT = Number(process.argv[2] || process.env.PORT || 8137);

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.txt': 'text/plain; charset=utf-8',
};

function safeJoin(urlPath) {
  // 防目录穿越: 解析后必须仍在 ROOT 内
  const resolved = resolve(join(ROOT, '.' + urlPath));
  const rootWithSep = ROOT.endsWith(sep) ? ROOT : ROOT + sep;
  if (resolved !== ROOT && !resolved.startsWith(rootWithSep)) return null;
  return resolved;
}

createServer(async (req, res) => {
  try {
    const reqURL = new URL(req.url, `http://127.0.0.1:${PORT}`);
    let pathname = decodeURIComponent(reqURL.pathname);
    if (pathname.endsWith('/')) pathname += 'index.html';

    const filePath = safeJoin(pathname);
    if (!filePath) {
      res.writeHead(403, { 'content-type': 'text/plain; charset=utf-8' });
      res.end('403 Forbidden');
      return;
    }

    const body = await readFile(filePath);
    res.writeHead(200, {
      'content-type': MIME[extname(filePath).toLowerCase()] || 'application/octet-stream',
      'cache-control': filePath.includes('data/trainers-') ? 'no-cache' : 'public, max-age=300',
    });
    res.end(body);
  } catch (err) {
    res.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' });
    res.end('404 Not Found');
  }
}).listen(PORT, '127.0.0.1', () => {
  console.log(`FLiNG 静态站已启动:  http://127.0.0.1:${PORT}/   (Ctrl+C 停止)`);
});
