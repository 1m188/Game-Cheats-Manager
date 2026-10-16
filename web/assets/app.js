/* FLiNG 修改器搜索 —— 纯静态前端 UI
 * 数据: globalThis.FLING_TRAINERS_DATA（由 web/scripts/build-index.mjs 生成，
 *       经 <script> 直接注入，无 fetch、无 ES module —— file:// 双击打开亦可工作）
 * 内核: globalThis.FlingSearch（assets/search.js，与 go 版行为对齐）
 * 时效: globalThis.FlingRefresh（assets/refresh.js）—— 页面打开时经 WP REST
 *       （实测对浏览器 CORS 开放）增量拉取基线之后的新修改器；失败静默回退离线索引
 */
(function () {
  'use strict';

  var data = globalThis.FLING_TRAINERS_DATA || null;
  var Search = globalThis.FlingSearch || null;
  var Refresh = globalThis.FlingRefresh || null;

  var els = {
    input: document.getElementById('search-input'),
    meta: document.getElementById('index-meta'),
    status: document.getElementById('status'),
    results: document.getElementById('results'),
  };

  var ORIGIN_LABELS = {
    fling_main: '官方主站',
    fling_archive: '历史归档 (2012–2019)',
  };

  var RUNTIME_PARAMS = { perPage: 50, maxPages: 8 };

  var debounceTimer = 0;  // 输入防抖计时器 id
  var runtimeMain = [];   // 在线增量合并出的条目（基线之后的新帖）
  var lastQuery = '';     // 最近一次搜索词, 增量刷新完成后用于重放

  function formatDate(iso) {
    try {
      return new Date(iso).toLocaleString('zh-CN', { hour12: false });
    } catch (e) {
      return iso;
    }
  }

  function collapseWhitespace(s) {
    return s.replace(/\s+/g, ' ');
  }

  function renderMeta() {
    if (!data) return;
    var counts = data.meta.counts;
    els.meta.innerHTML =
      '<span class="pill">基线 主站 ' + counts.main + '</span>' +
      '<span class="pill">归档 ' + counts.archive + '</span>' +
      '<span class="pill muted">构建于 ' + formatDate(data.meta.generated_at) + '</span>';
    if (runtimeMain.length) {
      appendPill('在线刷新 +' + runtimeMain.length + ' 条', 'pill');
    }
  }

  function appendPill(text, className) {
    var span = document.createElement('span');
    span.className = className + (className.indexOf('pill') === -1 ? ' pill' : '');
    span.textContent = text;
    els.meta.appendChild(span);
  }

  function setStatus(html) {
    els.status.innerHTML = html;
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function createCard(trainer) {
    var a = document.createElement('a');
    a.className = 'card';
    a.href = trainer.url;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    a.title = trainer.trainer_name + '\n' + trainer.url;

    var name = document.createElement('span');
    name.className = 'card-name';
    name.textContent = trainer.game_name;

    var badge = document.createElement('span');
    badge.className = 'badge ' + (trainer.origin === 'fling_archive' ? 'badge-archive' : 'badge-main');
    badge.textContent = ORIGIN_LABELS[trainer.origin] || trainer.origin;

    var open = document.createElement('span');
    open.className = 'card-open';
    open.textContent = '下载页 ↗';

    a.appendChild(name);
    a.appendChild(badge);
    a.appendChild(open);
    return a;
  }

  function renderResults(query, trainers) {
    els.results.innerHTML = '';
    if (!trainers.length) {
      setStatus('<span class="muted">没有找到与“' + escapeHtml(collapseWhitespace(query)) + '”匹配的修改器。</span>');
      return;
    }
    setStatus(
      '<span class="count">找到 <b>' + trainers.length + '</b> 条（按匹配质量 → 版本 → 名称排序）</span>'
    );
    var frag = document.createDocumentFragment();
    trainers.forEach(function (t) {
      frag.appendChild(createCard(t));
    });
    els.results.appendChild(frag);
  }

  // 主站列表 = 构建基线 + 运行时增量（在线刷新成功后才有增量）
  function buildMainList() {
    return runtimeMain.length ? data.main.concat(runtimeMain) : data.main;
  }

  function runSearch(query) {
    lastQuery = query;
    if (query === '') {
      setStatus('<span class="muted">输入游戏英文名开始搜索（至少 2 个字符）。</span>');
      els.results.innerHTML = '';
      return;
    }
    var trainers = Search.searchTrainers(data.archive, buildMainList(), query);
    renderResults(query, trainers);
  }

  // 运行时增量刷新: WP REST 实测对浏览器 CORS 开放; 失败时保持离线索引不动
  function initRuntimeRefresh() {
    if (!Refresh || !data || !data.meta || !data.meta.generated_at) return;
    var baselineURLs = new Set(data.main.map(function (t) { return t.url; }));

    Refresh.fetchNewPosts(globalThis.fetch, data.meta.generated_at, {
      perPage: RUNTIME_PARAMS.perPage,
      maxPages: RUNTIME_PARAMS.maxPages,
      baselineURLs: baselineURLs,
    })
      .then(function (items) {
        runtimeMain = items || [];
        if (runtimeMain.length) {
          renderMeta();
          if (lastQuery) runSearch(lastQuery); // 用户已经在搜索: 重放最新结果
        }
      })
      .catch(function () {
        appendPill('在线刷新不可用', 'pill muted');
      });
  }

  function onInput() {
    var value = els.input.value.trim();
    clearTimeout(debounceTimer);
    // 120ms 防抖：1221+ 条全量过滤成本极低
    debounceTimer = setTimeout(function () {
      runSearch(value);
    }, 120);
  }

  function onKeydown(e) {
    if (e.key === 'Escape') {
      els.input.value = '';
      runSearch('');
      els.input.focus();
    }
  }

  function onDocKeydown(e) {
    // "/" 聚焦搜索框（输入框已聚焦时不劫持）
    if (e.key === '/' && document.activeElement !== els.input) {
      e.preventDefault();
      els.input.focus();
      els.input.select();
    }
  }

  function showDataError() {
    els.meta.innerHTML = '';
    setStatus(
      '<span class="error">索引数据缺失：请先在 web/ 目录执行 <code>npm run build:index</code> 生成 data/trainers-data.js。</span>'
    );
  }

  function init() {
    if (!data || !Search) {
      showDataError();
      return;
    }
    els.meta.innerHTML = '';
    renderMeta();
    setStatus('<span class="muted">输入游戏英文名开始搜索（至少 2 个字符）。</span>');

    els.input.addEventListener('input', onInput);
    els.input.addEventListener('keydown', onKeydown);
    document.addEventListener('keydown', onDocKeydown);
    els.input.focus();
    initRuntimeRefresh();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
