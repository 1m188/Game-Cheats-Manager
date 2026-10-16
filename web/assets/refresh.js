/* FLiNG 修改器 —— 运行时增量刷新模块（纯逻辑 + HTTP，无 DOM 依赖）
 *
 * 为什么静态页能直接 fetch: WordPress REST 按核心设计会把请求 Origin 回显为
 * access-control-allow-origin（实测 : https://flingtrainer.com/wp-json 未关此特性），
 * 而站点自带的两个 HTML 列表页无 CORS —— 这也是"基线索引必须构建期生成"的原因。
 *
 * 数据通路:
 *   GET https://flingtrainer.com/wp-json/wp/v2/posts
 *       ?orderby=date&order=desc&after=<基线构建时间>&per_page=N&_fields=id,date,link,title
 *   返回增量 post; 分页直到短页或达到 maxPages 上限（达上限提示重建完整索引）。
 * 行为对齐: 标题截尾与 go/internal/fling/main_parser.go 相同（rsplit " Trainer"）;
 *           URL 去重、基线优先（构建索引先行，运行时只补后续新帖）。
 *
 * 说明: 这一模块使"数据过时"窗口从"上次构建"缩到"当下"; 失败时静默回退离线索引。
 */
(function () {
  'use strict';

  var ENDPOINT = 'https://flingtrainer.com/wp-json/wp/v2/posts';
  var REQUIRED_PREFIX = 'https://flingtrainer.com/';

  var ENTITIES = {
    '&amp;': '&',
    '&lt;': '<',
    '&gt;': '>',
    '&quot;': '"',
    '&#039;': "'",
    '&#39;': "'",
    '&apos;': "'",
    '&nbsp;': ' ',
  };

  function decodeEntities(text) {
    return text.replace(/&(?:amp|lt|gt|quot|#039|#39|apos|nbsp);/g, (m) => ENTITIES[m] ?? m);
  }

  // wp-json 的 post → 与构建索引同构的条目；字段不满意返回 null
  function normalizePost(post) {
    var raw = post && post.title && post.title.rendered ? String(post.title.rendered) : '';
    var title = decodeEntities(raw).replace(/\s+/g, ' ').trim();
    var link = post && post.link ? String(post.link) : '';

    // 与 main_parser.go:44-56 相同: 截掉最后一个 " Trainer"
    var idx = title.lastIndexOf(' Trainer');
    var gameName = idx > 0 ? title.slice(0, idx) : title;

    if (!gameName || link.indexOf(REQUIRED_PREFIX) !== 0) return null;
    return { game_name: gameName, url: link, origin: 'fling_main' };
  }

  function pageURL(after, perPage, page) {
    return (
      ENDPOINT +
      '?orderby=date&order=desc&after=' +
      encodeURIComponent(after) +
      '&per_page=' +
      perPage +
      '&_fields=id,date,link,title' +
      (page > 1 ? '&page=' + page : '')
    );
  }

  // 拉取基线构建时间之后的所有新帖。
  // opts:
  //     perPage            每页条数（WP 上限 100，默认 50）
  //     maxPages           最大翻页数，容量兜底（默认 8）
  //     baselineURLs       基线索引 URL 集合（可选；传入则过滤已在离线索引中的条目）
  // 返回 Promise<Trainer[]>; 网络/协议异常直接 reject，由调用方决定降级表现。
  async function fetchNewPosts(fetchImpl, after, opts) {
    if (typeof fetchImpl !== 'function') throw new Error('缺少 fetch 实现');
    if (!after) throw new Error('缺少基线时间 after');
    var perPage = opts && opts.perPage ? opts.perPage : 50;
    var maxPages = opts && opts.maxPages ? opts.maxPages : 8;
    var baselineURLs = opts && opts.baselineURLs ? opts.baselineURLs : null;

    var items = [];
    var seen = new Set();

    for (var page = 1; page <= maxPages; page++) {
      var res = await fetchImpl(pageURL(after, perPage, page), {
        headers: { accept: 'application/json' },
      });
      if (!res.ok) throw new Error('WP REST HTTP ' + res.status + ' @page ' + page);

      var json = await res.json();
      if (!Array.isArray(json)) throw new Error('WP REST 响应非数组 @page ' + page);

      json.forEach(function (post) {
        var t = normalizePost(post);
        if (!t) return;
        if (baselineURLs && baselineURLs.has(t.url)) return;
        if (seen.has(t.url)) return;
        seen.add(t.url);
        items.push(t);
      });

      // 短页 = 无更多数据
      if (json.length < perPage) break;
      if (page === maxPages) {
        // 达到翻页上限：增量远超预期，调用方应提示重建完整索引
        items.overflowed = true;
      }
    }

    return items;
  }

  globalThis.FlingRefresh = {
    ENDPOINT: ENDPOINT,
    normalizePost: normalizePost,
    fetchNewPosts: fetchNewPosts,
  };
})();
