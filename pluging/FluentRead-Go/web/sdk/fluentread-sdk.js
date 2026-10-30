/**
 * FluentRead 嵌入 SDK（P6，FR-901~906 + §7.9）。
 *
 * @file web/sdk/fluentread-sdk.js
 * 文件职责：一行 <script src="https://站点/sdk/fluentread-sdk.js" data-fr-...> 接入任意站点，
 * 提供悬浮球（可拖拽）、快捷翻译面板、划词译文气泡、点词查词卡、一键收藏（带原句）。
 * 全部 UI 渲染在 Shadow DOM 里与宿主页样式隔离（FR-904）；零依赖（FR-905 预算 30KB gzip）；
 * 只打 /api/sdk/*，来源白名单由服务端校验（FR-906）；与本站同一访客 cookie/配额（FR-903）。
 *
 * 配置（沿 §7.9）：data-fr-server-url / data-fr-from / data-fr-to / data-fr-theme，
 * 或 window.FluentReadConfig 同名字段。theme: light|dark。
 *
 * 模块边界：本文件自包含，不 import 任何东西；构建产物直接由 Go 服务下发。
 */
(function () {
  'use strict';

  // ------------------------------------------------------------------
  // 配置
  // ------------------------------------------------------------------
  var script = document.currentScript;
  var cfgRoot = window.FluentReadConfig || {};
  function cfg(name, fallback) {
    var v = script && script.getAttribute('data-fr-' + name);
    if (v === null || v === undefined || v === '') v = cfgRoot[name];
    return v === undefined || v === null || v === '' ? fallback : v;
  }
  var SERVER = cfg('server-url', '');
  if (!SERVER) {
    // serverUrl 缺省为 SDK 脚本自身的来源（同站部署）。
    try { SERVER = new URL(script.src).origin; } catch (e) { SERVER = ''; }
  }
  SERVER = String(SERVER).replace(/\/$/, '');
  var FROM = cfg('from', 'auto');
  var TO = cfg('to', 'zh-Hans');
  var THEME = cfg('theme', 'light') === 'dark' ? 'dark' : 'light';

  // ------------------------------------------------------------------
  // 样式 token（Shadow DOM 内生效；深浅两套，NFR-9 对比度同主站）
  // ------------------------------------------------------------------
  var CSS = [
    ':host{all:initial}',
    '*{box-sizing:border-box;font-family:-apple-system,"PingFang SC","Microsoft YaHei",sans-serif}',
    '.fr-ball{position:fixed;z-index:2147483000;width:44px;height:44px;border-radius:50%;',
    'background:#2b6cb0;color:#fff;font-size:12px;font-weight:700;display:flex;align-items:center;justify-content:center;',
    'cursor:grab;box-shadow:0 2px 10px rgba(0,0,0,.25);user-select:none;right:24px;bottom:24px}',
    '.fr-ball:active{cursor:grabbing}',
    '.fr-panel,.fr-bubble,.fr-card{position:fixed;z-index:2147483001;background:#fff;color:#1b1f23;',
    'border:1px solid #e4e6e9;border-radius:8px;box-shadow:0 4px 16px rgba(16,24,40,.16);font-size:13px;line-height:1.6}',
    '.fr-panel{width:320px;padding:12px}',
    '.fr-panel textarea{width:100%;height:84px;resize:vertical;border:1px solid #e4e6e9;border-radius:5px;',
    'padding:8px;font-size:13px;color:#1b1f23;background:#fff}',
    '.fr-panel .row{display:flex;gap:8px;margin-top:8px;align-items:center}',
    '.fr-btn{border:1px solid #2b6cb0;background:#2b6cb0;color:#fff;border-radius:5px;padding:4px 12px;',
    'font-size:12.5px;cursor:pointer}',
    '.fr-btn--ghost{background:#fff;color:#2b6cb0}',
    '.fr-out{margin-top:8px;padding:8px;background:#f0f2f5;border-radius:5px;white-space:pre-wrap;min-height:20px}',
    '.fr-bubble{max-width:420px;padding:10px 12px;white-space:pre-wrap}',
    '.fr-bubble .degraded{display:block;margin-top:6px;color:#9a6700;font-size:12px}',
    '.fr-card{width:min(320px,calc(100vw - 24px));padding:12px 14px}',
    '.fr-card .w{font-size:16px;font-weight:700}',
    '.fr-card .ph{color:#4c545b;margin-left:8px;font-size:12px}',
    '.fr-card .src{color:#9a6700;font-size:12px;margin-top:4px}',
    '.fr-card ul{margin:8px 0 0;padding-left:18px}',
    '.fr-card .div{border-top:1px solid #e4e6e9;margin:10px 0 8px}',
    '.fr-card .act{display:flex;gap:10px;align-items:center}',
    '.fr-card .col{border:1px solid #2b6cb0;color:#2b6cb0;background:#fff;border-radius:5px;',
    'font-size:12px;padding:2px 10px;cursor:pointer}',
    '.fr-card .done{color:#1a7f37;font-size:12px}',
    '.fr-err{color:#c0392b}',
    '.fr-skel{height:12px;background:#f0f2f5;border-radius:4px;margin:6px 0;overflow:hidden;position:relative}',
    '.fr-skel::after{content:"";position:absolute;inset:0;background:linear-gradient(90deg,transparent,rgba(255,255,255,.7),transparent);animation:frshine 1.2s infinite}',
    '@keyframes frshine{to{transform:translateX(100%)}}',
    '.fr-dark .fr-panel,.fr-dark .fr-bubble,.fr-dark .fr-card{background:#1d2126;color:#e8eaed;border-color:#33393f}',
    '.fr-dark .fr-panel textarea{background:#24292f;color:#e8eaed;border-color:#33393f}',
    '.fr-dark .fr-out{background:#24292f}',
    '.fr-dark .fr-skel{background:#24292f}',
    '.fr-dark .fr-btn--ghost{background:#1d2126}',
    '.fr-dark .fr-col{background:#1d2126}',
    '.fr-warn{position:fixed;z-index:2147483001;left:50%;transform:translateX(-50%);top:16px;',
    'background:#fff3d6;color:#9a6700;border:1px solid #e3b341;border-radius:8px;padding:8px 14px;font-size:13px}'
  ].join('');

  // ------------------------------------------------------------------
  // Shadow DOM 容器
  // ------------------------------------------------------------------
  var host = document.createElement('div');
  host.id = 'fluentread-sdk-host';
  var shadow = host.attachShadow({ mode: 'open' });
  var style = document.createElement('style');
  style.textContent = CSS;
  shadow.appendChild(style);
  var root = document.createElement('div');
  root.className = THEME === 'dark' ? 'fr-dark' : '';
  shadow.appendChild(root);
  document.documentElement.appendChild(host);

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  }

  function warn(msg) {
    var w = el('div', 'fr-warn', msg);
    root.appendChild(w);
    setTimeout(function () { w.remove(); }, 4000);
  }

  // ------------------------------------------------------------------
  // API（只打 /api/sdk/*；错误信封 {ok,error:{code,message}} 统一解包）
  // ------------------------------------------------------------------
  function api(path, body) {
    return fetch(SERVER + path, {
      method: body ? 'POST' : 'GET',
      headers: body ? { 'content-type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      credentials: 'include',
    }).then(function (res) {
      return res.json().catch(function () {
        throw { code: 'UPSTREAM_UNAVAILABLE', message: '服务暂时不可用，请稍后重试' };
      });
    }).then(function (payload) {
      if (!payload.ok) throw payload.error;
      return payload.data;
    });
  }

  function translateSegments(segments) {
    return api('/api/sdk/translate', { segments: segments, from: FROM, to: TO });
  }

  // ------------------------------------------------------------------
  // 悬浮球（可拖拽）+ 快捷翻译面板
  // ------------------------------------------------------------------
  var ball = el('div', 'fr-ball', '译');
  root.appendChild(ball);

  var panel = null;
  function togglePanel() {
    if (panel) { panel.remove(); panel = null; return; }
    panel = el('div', 'fr-panel');
    panel.style.right = '24px';
    panel.style.bottom = '80px';
    var ta = document.createElement('textarea');
    ta.placeholder = '输入要翻译的文字…';
    var out = el('div', 'fr-out');
    var row = el('div', 'row');
    var go = el('button', 'fr-btn', '翻译');
    var close = el('button', 'fr-btn fr-btn--ghost', '关闭');
    row.appendChild(go);
    row.appendChild(close);
    panel.appendChild(ta);
    panel.appendChild(out);
    panel.appendChild(row);
    root.appendChild(panel);

    go.addEventListener('click', function () {
      var text = ta.value.trim();
      if (!text) return;
      out.textContent = '';
      out.appendChild(el('div', 'fr-skel'));
      out.appendChild(el('div', 'fr-skel'));
      var segs = text.split(/\n\s*\n/).filter(function (s) { return s.trim(); }).slice(0, 20)
        .map(function (s) { return { text: s }; });
      translateSegments(segs).then(function (data) {
        out.textContent = '';
        data.segments.forEach(function (seg) {
          if (seg.status === 'ok') out.appendChild(el('div', '', seg.text));
          else if (seg.status === 'failed') out.appendChild(el('div', 'fr-err', seg.message || '翻译失败'));
        });
      }).catch(function (err) {
        out.textContent = '';
        out.appendChild(el('span', 'fr-err', err.message || '翻译失败，请稍后重试'));
      });
    });
    close.addEventListener('click', togglePanel);
  }

  ball.addEventListener('click', function () {
    if (dragMoved) { dragMoved = false; return; }
    togglePanel();
  });

  // 拖拽（区分点击与拖动）。
  var dragMoved = false;
  ball.addEventListener('mousedown', function (e) {
    var startX = e.clientX, startY = e.clientY;
    var rect = ball.getBoundingClientRect();
    var offsetX = startX - rect.left, offsetY = startY - rect.top;
    dragMoved = false;
    function move(ev) {
      if (Math.abs(ev.clientX - startX) + Math.abs(ev.clientY - startY) > 4) dragMoved = true;
      ball.style.left = Math.max(0, ev.clientX - offsetX) + 'px';
      ball.style.top = Math.max(0, ev.clientY - offsetY) + 'px';
      ball.style.right = 'auto';
      ball.style.bottom = 'auto';
    }
    function up() {
      document.removeEventListener('mousemove', move);
      document.removeEventListener('mouseup', up);
    }
    document.addEventListener('mousemove', move);
    document.addEventListener('mouseup', up);
  });

  // ------------------------------------------------------------------
  // 划词译文气泡（FR-902：选中即看中文）
  // ------------------------------------------------------------------
  var bubble = null;
  document.addEventListener('mouseup', function (e) {
    // 同上：Shadow UI 内的 mouseup（target=host）不算宿主页的划词。
    if (e.target === host || host.contains(e.target)) return;
    var sel = document.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) return;
    var text = sel.toString().trim();
    if (text.length < 2 || text.length > 2000) return;
    var rect = sel.getRangeAt(0).getBoundingClientRect();
    if (bubble) bubble.remove();
    bubble = el('div', 'fr-bubble');
    bubble.style.left = Math.max(8, rect.left) + 'px';
    bubble.style.top = Math.min(window.innerHeight - 60, rect.bottom + 8) + 'px';
    bubble.appendChild(el('div', 'fr-skel'));
    root.appendChild(bubble);

    var segs = text.split(/\n\s*\n/).filter(function (s) { return s.trim(); }).slice(0, 20)
      .map(function (s) { return { text: s }; });
    translateSegments(segs).then(function (data) {
      bubble.textContent = '';
      var failed = 0;
      data.segments.forEach(function (seg) {
        if (seg.status === 'ok') bubble.appendChild(el('div', '', seg.text));
        else failed++;
      });
      if (failed) bubble.appendChild(el('span', 'fr-err', failed + ' 段未翻译成功'));
      if (data.degraded) bubble.appendChild(el('span', 'degraded', '本站今日额度紧张，已用免费方式完成'));
    }).catch(function (err) {
      bubble.textContent = '';
      bubble.appendChild(el('span', 'fr-err', err.message || '翻译失败，请稍后重试'));
    });
  });

  document.addEventListener('mousedown', function (e) {
    // 注意：Shadow UI 内的点击在 document 上重定向为 host（e.target === host），
    // 用 host.contains 判断而不是 shadow.contains。
    if (bubble && e.target !== host && !host.contains(e.target)) { bubble.remove(); bubble = null; }
  }, true);

  // ------------------------------------------------------------------
  // 点词查词卡 + 收藏（FR-902：点词看释义；收藏进同一个词表 FR-1201）
  // ------------------------------------------------------------------
  var card = null;
  document.addEventListener('dblclick', function (e) {
    if (shadow.contains(e.target)) return;
    var sel = document.getSelection();
    var word = sel && !sel.isCollapsed ? sel.toString().trim() : '';
    if (!word || !/^[a-zA-Z][a-zA-Z'\-]*$/.test(word)) return;
    var rect = sel.getRangeAt(0).getBoundingClientRect();
    // 查词瞬间提取所在句（8.5），供收藏时带上原句。
    var contextSentence = '';
    try {
      var text = sel.getRangeAt(0).startContainer.textContent || '';
      var pos0 = text.toLowerCase().indexOf(word.toLowerCase());
      if (pos0 >= 0) {
        var before = text.slice(0, pos0 + word.length);
        var after = text.slice(pos0 + word.length);
        var st = Math.max(before.lastIndexOf('.'), before.lastIndexOf('!'), before.lastIndexOf('?')) + 1;
        var en = after.search(/[.!?]/);
        contextSentence = (before.slice(st) + (en >= 0 ? after.slice(0, en + 1) : after)).trim().slice(0, 300);
      }
    } catch (err) { contextSentence = ''; }
    if (card) card.remove();
    card = el('div', 'fr-card');
    card.style.left = Math.max(8, Math.min(rect.left, window.innerWidth - 340)) + 'px';
    card.style.top = Math.min(window.innerHeight - 200, rect.bottom + 8) + 'px';
    card.appendChild(el('div', 'w', word));
    for (var i = 0; i < 2; i++) card.appendChild(el('div', 'fr-skel'));
    root.appendChild(card);

    api('/api/sdk/word?word=' + encodeURIComponent(word.toLowerCase())).then(function (entry) {
      card.textContent = '';
      var head = el('div', '');
      head.appendChild(el('span', 'w', entry.word));
      if (entry.phonetic) head.appendChild(el('span', 'ph', entry.phonetic));
      card.appendChild(head);
      if (entry.word !== entry.query) {
        card.appendChild(el('div', 'src', 'ⓘ 查询的是 ' + entry.query + ' —— 原形 ' + entry.word));
      }
      if (entry.source === 'model') card.appendChild(el('div', 'src', 'ⓘ 在线解释（AI 生成，仅供参考）'));
      var ul = el('ul', '');
      entry.senses.forEach(function (s) {
        ul.appendChild(el('li', '', (s.pos ? s.pos + ' ' : '') + s.def));
      });
      card.appendChild(ul);
      card.appendChild(el('div', 'div'));
      var act = el('div', 'act');
      var col = el('button', 'col', entry.collected.yes ? '✓ 已收藏' : '☆ 收藏');
      if (entry.collected.yes) col.className = 'done';
      col.addEventListener('click', function () {
        // 原句在查词瞬间已由选区确定：点击时选区可能已被浏览器收起。
        col.disabled = true;
        col.textContent = '收藏中…';
        api('/api/sdk/word', { word: entry.word, lang: 'en', contextSentence: contextSentence }).then(function () {
          col.className = 'done';
          col.textContent = '✓ 已收藏';
        }).catch(function (err) {
          col.disabled = false;
          col.textContent = '☆ 收藏';
          warn(err.message || '收藏失败，请稍后重试');
        });
      });
      act.appendChild(col);
      card.appendChild(act);
    }).catch(function (err) {
      card.textContent = '';
      card.appendChild(el('span', 'fr-err', err.code === 'ORIGIN_FORBIDDEN'
        ? '本站点未开通翻译服务，请联系站长添加域名'
        : (err.message || '没查到这个词')));
    });
  });

  document.addEventListener('mousedown', function (e) {
    if (card && e.target !== host && !host.contains(e.target)) { card.remove(); card = null; }
  }, true);
})();
