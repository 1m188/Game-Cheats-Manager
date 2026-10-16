#!/usr/bin/env node
// web/scripts/build-index.mjs —— 构建纯静态索引
//
// 数据源（与 go/internal/fling/index.go 相同的两个页面）:
//   1. https://flingtrainer.com/all-trainers/      —— 主站 A-Z 列表（旧地址 /all-trainers-a-z/ 已 301 到这里）
//   2. https://archive.flingtrainer.com/            —— 2012–2019 历史归档
//
// 产物:
//   web/data/trainers-data.js —— 纯静态页面直接 <script> 加载（兼容 file:// 双击打开）
//   web/data/trainers.json    —— 同一份内容的 JSON（供其他工具或未来构建链引用）
//
// 解析/清洗规则逐条对齐 go/internal/fling:
//   - main    解析: div.letter-section > ul > li > a，链接文本截掉最后一个 " Trainer"（main_parser.go:40-56）
//   - archive 解析: a[target="_self"]，版本后缀正则清除 + "_"→": "（archive_parser.go:22-96）
//   - 忽略列表 4 个游戏；"Bright.Memory.Episode.1" 特例（archive_parser.go:26-78）
//
// 运行:
//   node web/scripts/build-index.mjs
//   （本机网络直连即可；若所在网络强制走代理且 Node>=24，可用: NODE_USE_ENV_PROXY=1 node ...）

import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// 与 go/internal/store/cache.go:18 相同的 User-Agent
const USER_AGENT =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110 Safari/537.36';

const MAIN_URL = 'https://flingtrainer.com/all-trainers/';
const ARCHIVE_URL = 'https://archive.flingtrainer.com/';
const MAIN_BASE = 'https://flingtrainer.com/';

const ORIGIN_MAIN = 'fling_main';
const ORIGIN_ARCHIVE = 'fling_archive';

const HERE = dirname(fileURLToPath(import.meta.url));
const DATA_DIR = join(HERE, '..', 'data');

// ---------- 与 go/internal/fling/archive_parser.go 完全一致的常量 ----------

// 版本后缀去除正则，复制自 archive_parser.go:22（Python download_display_thread.py:166）
// JS 需带 /g 以等价 Go ReplaceAllString 的全部替换语义
const VERSION_REMOVAL_RE =
  / v[\d.]+.*|\.\bv.*| \d+\.\d+\.\d+.*| Plus\s\d+.*|Build\s\d+.*|(\d+\.\d+-Update.*)|Update\s\d+.*|\(Update\s.*| Early Access .*|\.Early.Access.*/g;

// 忽略列表，复制自 archive_parser.go:26-31（Python download_display_thread.py:168-173）
const IGNORED_TRAINERS = new Set([
  'Dying Light The Following Enhanced Edition',
  'Monster Hunter World',
  'Street Fighter V',
  'World War Z',
]);

const ENTITIES = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#39;': "'",
  '&apos;': "'",
  '&nbsp;': ' ',
};

// goquery 内部会解码 HTML 实体，这里保持一致
function decodeEntities(text) {
  return text.replace(/&(?:amp|lt|gt|quot|#39|apos|nbsp);/g, (m) => ENTITIES[m] ?? m);
}

// anchor 内文本：去标签 + 实体解码（goquery link.Text() 等价）
function anchorText(rawHtml) {
  return decodeEntities(rawHtml.replace(/<[^>]*>/g, ''));
}

async function fetchText(url, label) {
  const resp = await fetch(url, { headers: { 'user-agent': USER_AGENT } });
  if (!resp.ok) {
    throw new Error(`抓取${label}失败: HTTP ${resp.status} — ${url}（检查网络，或参考 README 的代理说明）`);
  }
  return resp.text();
}

// ---------- main 解析：对齐 main_parser.go ----------

// 主站列表页给绝对 href；若将来回退相对路径，拼主站 base（main_parser.go:58-67）
function joinMainURL(href) {
  if (href.startsWith('/')) {
    try {
      return new URL(href, MAIN_BASE).href;
    } catch {
      return href;
    }
  }
  return href;
}

// 解析所有 div.letter-section 区块内的 a 链接
export function parseMainHTML(html) {
  const trainers = [];
  const sectionRe = /<[a-z]+[^>]*\bclass=["'][^"']*letter-section[^"']*["'][^>]*>/gi;
  const starts = [...html.matchAll(sectionRe)].map((m) => m.index);
  const ranges = starts.map((s, i) => [s, i + 1 < starts.length ? starts[i + 1] : html.length]);
  const anchorRe = /<a\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>([\s\S]*?)<\/a>/gi;

  for (const [from, to] of ranges) {
    const chunk = html.slice(from, to);
    for (const m of chunk.matchAll(anchorRe)) {
      const href = decodeEntities(m[1]).trim();
      // 站点适配: 现版 A-Z Listing 每个 letter-section 顶部有页内字母导航锚点
      // (href="#a-z-listing-*"),当初 go 版抓取的旧页面没有这种元素，予以跳过
      if (href.startsWith('#')) continue;
      const rawName = anchorText(m[2]).trim();

      // rsplit(" Trainer", 1)[0] 的等价实现（main_parser.go:44-56）
      const idx = rawName.lastIndexOf(' Trainer');
      const gameName = idx > 0 ? rawName.slice(0, idx) : rawName;
      if (!gameName) continue;

      trainers.push({
        game_name: gameName,
        url: joinMainURL(href),
        origin: ORIGIN_MAIN,
      });
    }
  }
  return trainers;
}

// ---------- archive 解析：对齐 archive_parser.go ----------

export function parseArchiveHTML(html) {
  const trainers = [];
  // Python: archiveData.find_all(target="_self")（archive_parser.go:62）
  const anchorRe =
    /<a\b[^>]*\btarget\s*=\s*["']_self["'][^>]*>([\s\S]*?)<\/a>/gi;

  for (const m of html.matchAll(anchorRe)) {
    const tag = m[0];
    const hrefM = tag.match(/\bhref\s*=\s*["']([^"']+)["']/i);
    if (!hrefM) continue;
    const href = decodeEntities(hrefM[1]).trim();
    const rawName = anchorText(m[1]);

    // 1. 去版本后缀（仅用于忽略列表匹配，archive_parser.go:66-69）
    let strippedName = rawName.replace(VERSION_REMOVAL_RE, '');
    strippedName = strippedName.replaceAll('_', ': ').trim();

    // 2. GameName 保留版本信息（archive_parser.go:71-73）
    let gameName = rawName.replaceAll('_', ': ').trim();

    // 3. Bright Memory 特例（archive_parser.go:75-78）
    if (strippedName === 'Bright.Memory.Episode.1') {
      gameName = 'Bright Memory: Episode 1';
    }

    // 4. 忽略列表（archive_parser.go:80-83）
    if (IGNORED_TRAINERS.has(strippedName)) continue;

    const url = new URL(href, ARCHIVE_URL).href;
    trainers.push({
      game_name: gameName,
      url,
      origin: ORIGIN_ARCHIVE,
    });
  }
  return trainers;
}

// ---------- 主流程 ----------

// 源内按 URL 去重,保留最后一次出现(与 go search.go 主站循环的"后写覆盖"一致;
// 站点现版会把同一作列进多个字母区块,搜索结果等价,索引更小)
function dedupeByURL(trainers) {
  const map = new Map();
  for (const t of trainers) map.set(t.url, t);
  const deduped = [...map.values()];
  const removed = trainers.length - deduped.length;
  if (removed > 0) console.log(`  去重: ${removed} 条同 URL 重复项被合并`);
  return deduped;
}

async function main() {
  console.log('[1/3] 抓取两个数据源页面…');
  const [archiveHTML, mainHTML] = await Promise.all([
    fetchText(ARCHIVE_URL, 'archive 归档页'),
    fetchText(MAIN_URL, '主站列表页'),
  ]);

  console.log('[2/3] 解析索引…');
  const archive = dedupeByURL(parseArchiveHTML(archiveHTML));
  const main = dedupeByURL(parseMainHTML(mainHTML));

  // 结构漂移预警：解析条数骤降说明站点改版，先报错而不是生成残缺索引
  // （live 实测约 main≈700+ / archive≈460+，阈值留足余量）
  if (main.length < 100) {
    throw new Error(`主站解析结果异常（${main.length} 条）；站点结构可能已变更，需更新 parseMainHTML`);
  }
  if (archive.length < 50) {
    throw new Error(`归档页解析结果异常（${archive.length} 条）；站点结构可能已变更，需更新 parseArchiveHTML`);
  }

  const data = {
    meta: {
      generated_at: new Date().toISOString(),
      format: 1,
      sources: [MAIN_URL, ARCHIVE_URL],
      counts: { main: main.length, archive: archive.length, total: main.length + archive.length },
      note: '由 web/scripts/build-index.mjs 自动生成，解析规则对齐 go/internal/fling',
    },
    main,
    archive,
  };

  console.log('[3/3] 写出数据文件…');
  await mkdir(DATA_DIR, { recursive: true });

  // data JS：classic script 全局赋值，file:// 双击 index.html 也可用（无 fetch、无 module）
  const jsHeader =
    '/* eslint-disable */\n' +
    '// ⚠️ 本文件由 web/scripts/build-index.mjs 自动生成，请勿手改。刷新索引: npm run build:index\n' +
    '// 数据字段对齐 go(internal/fling/models.go) Trainer 结构: game_name / url / origin\n';
  const jsBody = `globalThis.FLING_TRAINERS_DATA = ${JSON.stringify(data)};\n`;
  await writeFile(join(DATA_DIR, 'trainers-data.js'), jsHeader + jsBody, 'utf8');
  await writeFile(join(DATA_DIR, 'trainers.json'), JSON.stringify(data), 'utf8');

  console.log(`完成 ✓ 主站 ${main.length} 条 + 归档 ${archive.length} 条 = ${data.meta.counts.total} 条`);
  console.log(`数据更新时间: ${data.meta.generated_at}`);
  console.log('输出: web/data/trainers-data.js, web/data/trainers.json');
}

main().catch((err) => {
  console.error(`构建失败: ${err.message}`);
  process.exit(1);
});
