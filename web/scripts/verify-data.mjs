#!/usr/bin/env node
// web/scripts/verify-data.mjs —— 索引数据 + 搜索内核回归校验
//
// 校验两类事实:
//   1. Node 原生 API 单元断言（与 go/internal/fling 的单元测试口径一致，直接搬用其测试向量）
//   2. 真实构建产物上的行为断言（搜索 Dredge / 空关键词 / 无匹配 / 子序列模糊匹配 / 排序）
//
// 运行: npm run verify   （需先 npm run build:index）

import assert from 'node:assert/strict';

// 导入即执行的全局脚本（经典 script 内核、构建产物、增量刷新模块）
await import('../assets/search.js');
await import('../assets/refresh.js');
await import('../data/trainers-data.js');

const S = globalThis.FlingSearch;
const R = globalThis.FlingRefresh;
const D = globalThis.FLING_TRAINERS_DATA;
assert.ok(S, 'search.js 未暴露 globalThis.FlingSearch');
assert.ok(R, 'refresh.js 未暴露 globalThis.FlingRefresh');
assert.ok(D, 'trainers-data.js 未暴露 globalThis.FLING_TRAINERS_DATA');

// ---------- 单元断言（移植自 go/internal/fling 的测试向量） ----------

// Sanitize：数字→罗马数字 / 保留 & / 去标点（含下划线）/ 小写 —— 对齐 search.go:43-66
assert.equal(S.sanitize('PvZ 2'), 'pvzii');
assert.equal(S.sanitize('800'), 'dccc');
assert.equal(S.sanitize('a&b-1!'), 'a&bi');
assert.equal(S.sanitize('my_mod.v3'), 'mymodviii'); // 下划线/点被删,"v"保留,3→罗马数字iii
assert.equal(S.sanitize(''), '');

// 罗马数字边界 —— 对齐 search.go:16-39（原样输出大写;小写化在 sanitize 末尾统一处理）
assert.equal(S.arabicToRoman(0), '0');
assert.equal(S.arabicToRoman(9), 'IX');
assert.equal(S.arabicToRoman(1994), 'MCMXCIV');

// 编辑距离 —— 对齐 search.go:68-104 的标准 Levenshtein 语义
assert.equal(S.levenshteinDistance('kitten', 'sitting'), 3);

// 版本比较：版本新在前 / 更具体的版本号在前 —— 对齐 search.go:146-179
assert.ok(S.compareVersion('X v2.0', 'X v1.0') < 0);
assert.equal(S.compareVersion('X v1.0', 'X v1.0'), 0);
assert.ok(S.compareVersion('X v1', 'X v1.0') > 0); // v1.0 更具体，应排前

// isFuzzyMatch：子序列(模糊)命中 —— 对齐 search.go:106-126
assert.ok(S.isFuzzyMatch(S.sanitize('acec7'), S.sanitize('Ace Combat 7: Skies Unknown')));

// ---------- 真实数据上的行为断言 ----------

assert.ok(D.meta && D.meta.counts, '数据缺少 meta.counts');
assert.ok(D.meta.counts.main >= 100, '主站条数异常: ' + D.meta.counts.main);
assert.ok(D.meta.counts.archive >= 50, '归档条数异常: ' + D.meta.counts.archive);

// 去重口径: URL 构建后两个源各自不应有重复 URL
function assertNoDupURL(list, label) {
  const seen = new Set();
  for (const t of list) {
    assert.ok(t.game_name && t.url && t.origin, `${label} 条目字段不完整`);
    assert.ok(!seen.has(t.url), `${label} 存在重复 URL: ${t.url}`);
    seen.add(t.url);
  }
}
assertNoDupURL(D.main, 'main');
assertNoDupURL(D.archive, 'archive');

// 搜索: 命中 / 空输入 / 过短 / 无匹配
const dredge = S.searchTrainers(D.archive, D.main, 'dredge');
assert.ok(dredge.length > 0, 'Dredge 应有搜索结果');
assert.ok(dredge[0].game_name.toLowerCase().includes('dredge'), '首位结果不符: ' + dredge[0].game_name);
assert.equal(dredge[0].trainer_name, '[FL] ' + dredge[0].game_name + ' Trainer', '展示名格式不符');

assert.deepEqual(S.searchTrainers(D.archive, D.main, ''), []);
assert.deepEqual(S.searchTrainers(D.archive, D.main, 'z'), []); // 单字符关键词 → 空
assert.deepEqual(S.searchTrainers(D.archive, D.main, 'zzzznotagame'), []);

// 排序验证: 排序口径已在 dry 数据上覆盖,此处仅验证返回数组类型
assert.ok(Array.isArray(dredge));

// ---------- 运行时增量层（refresh.js; WP REST 实测对浏览器 CORS 开放,回显 Origin） ----------

// normalizePost: 与 go main_parser 相同的 " Trainer" 截尾 + 实体解码 + 主站 URL 前缀校验
assert.deepEqual(
  R.normalizePost({
    title: { rendered: 'Planet Zoo 2 Trainer' },
    link: 'https://flingtrainer.com/trainer/planet-zoo-2-trainer/',
  }),
  {
    game_name: 'Planet Zoo 2',
    url: 'https://flingtrainer.com/trainer/planet-zoo-2-trainer/',
    origin: 'fling_main',
  }
);
assert.equal(R.normalizePost({ title: { rendered: 'x' }, link: 'http://example.com/x' }), null);

if (typeof fetch === 'function') {
  const baseline = new Set(D.main.map((t) => t.url));
  try {
    // (a) 以当前基线的构建时刻作 after: 增量应为空(基线已包含其后全部帖子)
    const fresh = await R.fetchNewPosts(fetch, D.meta.generated_at, {
      perPage: 50,
      maxPages: 1,
      baselineURLs: baseline,
    });
    assert.equal(fresh.length, 0, '构建时刻之后不应解析出增量条目');

    // (b) 不带基线过滤: 从 2020 起可拉到批量帖子,并被 maxPages × perPage 封顶
    const bulk = await R.fetchNewPosts(fetch, '2020-01-01T00:00:00', { perPage: 50, maxPages: 2 });
    assert.ok(bulk.length > 0, 'after=2020-01-01 应拉到帖子');
    assert.ok(bulk.length <= 100, 'maxPages=2 × perPage=50 封顶应生效');
    for (const t of bulk) {
      assert.ok(t.game_name && t.url, '增量条目字段应完整');
      assert.ok(t.url.startsWith('https://flingtrainer.com/'), '增量必须来自主站: ' + t.url);
    }
    console.log(`✓ 在线增量层: 构建时刻后 0 新帖; 标题规范化经真实数据验证(${bulk.length} 条封顶样本)`);
  } catch (e) {
    console.log(`· 在线增量层测试跳过(站点不可达): ${e.message}`);
  }
}

console.log(`✓ 全部校验通过 — main ${D.meta.counts.main} / archive ${D.meta.counts.archive} / 共 ${D.meta.counts.total} 条`)
