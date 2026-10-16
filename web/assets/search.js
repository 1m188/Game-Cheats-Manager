/* FLiNG 修改器搜索内核（纯函数，无 DOM 依赖）
 *
 * 与 go/internal/fling/search.go 逐函数对齐：
 *   - sanitize          ← search.go:43-66（数字→罗马数字 + 去标点保 & + 小写）
 *   - levenshteinDistance ← search.go:68-104（滚动两行的标准编辑距离）
 *   - isFuzzyMatch      ← search.go:106-126（子序列 Match → 子串 → Levenshtein ≤ len/3）
 *   - matchQuality      ← search.go:128-144（0 精确 /1 前缀 /2 子串 /3 子序列 /4 编辑距离）
 *   - compareVersion    ← search.go:146-179（v1.2.3 解析与降序比较）
 *   - searchTrainers    ← search.go:181-247（URL 去重、main 覆盖 archive、三级排序）
 *
 * 通过 classic <script> 暴露 globalThis.FlingSearch，兼容 file:// 直接打开。
 * Node 侧可 require/import 本文件后用 globalThis.FlingSearch 做测试。
 */
(function () {
  'use strict';

  // 罗马数字映射表（search.go:21-28）
  var ROMAN_TABLE = [
    [1000, 'M'], [900, 'CM'], [500, 'D'], [400, 'CD'],
    [100, 'C'], [90, 'XC'], [50, 'L'], [40, 'XL'],
    [10, 'X'], [9, 'IX'], [5, 'V'], [4, 'IV'], [1, 'I'],
  ];

  // arabicToRoman: 0 → "0"；≥1 逐档消减（search.go:16-39，循环本身不设上限，4999 → "MMMMCMXCIX"）
  function arabicToRoman(n) {
    if (n === 0) return '0';
    var result = '';
    var remaining = n;
    for (var i = 0; i < ROMAN_TABLE.length; i++) {
      var value = ROMAN_TABLE[i][0];
      var symbol = ROMAN_TABLE[i][1];
      while (remaining >= value) {
        result += symbol;
        remaining -= value;
      }
    }
    return result;
  }

  // 数字→罗马数字（无法解析则原样保留）
  function numbersToRoman(s) {
    return s.replace(/\d+/g, function (m) {
      var n = parseInt(m, 10);
      if (!Number.isFinite(n)) return m;
      try {
        return arabicToRoman(n);
      } catch (e) {
        return m;
      }
    });
  }

  // Sanitize：数字转罗马数字 → 去除标点（保留 &，同 Python string.punctuation 含下划线）→ 小写
  // 对齐 search.go:43-66 的 [^a-zA-Z0-9&] 正则
  function sanitize(s) {
    if (s == null || s === '') return '';
    var replaced = numbersToRoman(s);
    var cleaned = replaced.replace(/[^a-zA-Z0-9&]/g, '');
    return cleaned.toLowerCase();
  }

  // levenshteinDistance：两个一维滚动数组，O(min(m,n)) 空间（search.go:68-104）
  function levenshteinDistance(a, b) {
    if (a.length > b.length) {
      var tmp = a;
      a = b;
      b = tmp;
    }
    var aLen = a.length;
    var bLen = b.length;
    var prev = new Array(aLen + 1);
    var curr = new Array(aLen + 1);
    var i, j;
    for (i = 0; i <= aLen; i++) prev[i] = i;
    for (j = 1; j <= bLen; j++) {
      curr[0] = j;
      for (i = 1; i <= aLen; i++) {
        var cost = a.charCodeAt(i - 1) === b.charCodeAt(j - 1) ? 0 : 1;
        curr[i] = Math.min(prev[i] + 1, curr[i - 1] + 1, prev[i - 1] + cost);
      }
      var swap = prev;
      prev = curr;
      curr = swap;
    }
    return prev[aLen];
  }

  // 子序列匹配：pattern 的字符依序出现在 str 中（等价 lithammer/fuzzysearch Match；
  // Go 版在已小写的字符串上调用，大小写敏感问题不存在）
  function isSubsequence(pattern, str) {
    if (pattern.length === 0) return true;
    var pi = 0;
    for (var i = 0; i < str.length; i++) {
      if (str.charCodeAt(i) === pattern.charCodeAt(pi)) {
        pi++;
        if (pi === pattern.length) return true;
      }
    }
    return false;
  }

  // isFuzzyMatch：fuzzy.Match → Contains → Levenshtein ≤ max(1, ⌊len/3⌋)（search.go:106-126）
  function isFuzzyMatch(sanitizedKeyword, sanitizedGameName) {
    if (isSubsequence(sanitizedKeyword, sanitizedGameName)) return true;
    if (sanitizedGameName.indexOf(sanitizedKeyword) !== -1) return true;
    var threshold = Math.floor(sanitizedKeyword.length / 3);
    if (threshold < 1) threshold = 1;
    return levenshteinDistance(sanitizedKeyword, sanitizedGameName) <= threshold;
  }

  // matchQuality：值越小越精确（search.go:128-144）
  function matchQuality(sanitizedKeyword, sanitizedGameName) {
    if (sanitizedKeyword === sanitizedGameName) return 0;
    if (sanitizedGameName.startsWith(sanitizedKeyword)) return 1;
    if (sanitizedGameName.indexOf(sanitizedKeyword) !== -1) return 2;
    if (isSubsequence(sanitizedKeyword, sanitizedGameName)) return 3;
    return 4;
  }

  // parseVersion：从游戏名提取 v1.2.3（大小写敏感，与 Go 正则一致），无版本 → [0]（search.go:146-167）
  var VERSION_RE = /v(\d+)(?:\.(\d+))?(?:\.(\d+))?/;

  function parseVersion(name) {
    var m = VERSION_RE.exec(name);
    if (m === null) return [0];
    var v = [];
    for (var g = 1; g < m.length; g++) {
      if (m[g] === undefined || m[g] === '') break;
      v.push(parseInt(m[g], 10));
    }
    if (v.length === 0) return [0];
    return v;
  }

  // compareVersion：返回 -1/0/1；版本新的在前（search.go:169-179）
  function compareVersion(a, b) {
    var va = parseVersion(a);
    var vb = parseVersion(b);
    for (var i = 0; i < va.length && i < vb.length; i++) {
      if (va[i] !== vb[i]) return vb[i] - va[i];
    }
    return vb.length - va.length;
  }

  function cmpString(x, y) {
    if (x === y) return 0;
    return x < y ? -1 : 1;
  }

  // displayTrainerName("[FL] {GameName} Trainer"（search.go:222-225）
  function displayTrainerName(gameName) {
    return '[FL] ' + gameName + ' Trainer';
  }

  // SearchTrainers：archive 先入、main 同 URL 覆盖；排序 = 匹配质量 → 版本降序 → 名称字母序
  // 对齐 search.go:181-247
  function searchTrainers(archiveList, mainList, keyword) {
    if (!keyword) return [];
    var sanitizedKeyword = sanitize(keyword);
    if (sanitizedKeyword.length < 2) return [];

    var byURL = new Map();
    var quality = new Map();

    (archiveList || []).forEach(function (t) {
      var key = sanitize(t.game_name);
      if (key.length < 2) return;
      if (!byURL.has(t.url) && isFuzzyMatch(sanitizedKeyword, key)) {
        byURL.set(t.url, {
          game_name: t.game_name,
          url: t.url,
          origin: t.origin,
          trainer_name: displayTrainerName(t.game_name),
        });
        quality.set(t.url, matchQuality(sanitizedKeyword, key));
      }
    });

    (mainList || []).forEach(function (t) {
      var key = sanitize(t.game_name);
      if (key.length < 2) return;
      if (isFuzzyMatch(sanitizedKeyword, key)) {
        byURL.set(t.url, {
          game_name: t.game_name,
          url: t.url,
          origin: t.origin,
          trainer_name: displayTrainerName(t.game_name),
        });
        quality.set(t.url, matchQuality(sanitizedKeyword, key));
      }
    });

    var results = Array.from(byURL.values());
    results.sort(function (a, b) {
      var qa = quality.get(a.url);
      var qb = quality.get(b.url);
      if (qa !== qb) return qa - qb;
      var cv = compareVersion(a.game_name, b.game_name);
      if (cv !== 0) return cv;
      return cmpString(a.game_name.toLowerCase(), b.game_name.toLowerCase());
    });
    return results;
  }

  globalThis.FlingSearch = {
    arabicToRoman: arabicToRoman,
    sanitize: sanitize,
    levenshteinDistance: levenshteinDistance,
    isFuzzyMatch: isFuzzyMatch,
    matchQuality: matchQuality,
    compareVersion: compareVersion,
    searchTrainers: searchTrainers,
  };
})();
