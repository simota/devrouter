// DevRouter LP — minimal interactivity
// 1) language toggle (JP <-> EN) via class on <html>
// 2) copy buttons for terminal blocks
// 3) local clock for footer

(function () {
  'use strict';

  const root = document.documentElement;
  const STORAGE_KEY = 'devrouter-lang';

  // ---------- language toggle ----------
  function getInitialLang() {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved === 'jp' || saved === 'en') return saved;
    } catch (_) {
      /* ignore */
    }
    const browser = (navigator.language || 'ja').toLowerCase();
    return browser.startsWith('ja') ? 'jp' : 'jp'; // default JP per brief
  }

  function setLang(lang) {
    root.classList.remove('lang-jp', 'lang-en');
    root.classList.add('lang-' + lang);
    root.setAttribute('lang', lang === 'jp' ? 'ja' : 'en');
    try {
      localStorage.setItem(STORAGE_KEY, lang);
    } catch (_) {
      /* ignore */
    }
    document.querySelectorAll('[data-lang-btn]').forEach((btn) => {
      btn.classList.toggle('active', btn.getAttribute('data-lang-btn') === lang);
      btn.setAttribute('aria-pressed', String(btn.getAttribute('data-lang-btn') === lang));
    });
  }

  function initLang() {
    setLang(getInitialLang());
    document.querySelectorAll('[data-lang-btn]').forEach((btn) => {
      btn.addEventListener('click', () => setLang(btn.getAttribute('data-lang-btn')));
    });
  }

  // ---------- copy buttons ----------
  function initCopyButtons() {
    document.querySelectorAll('[data-copy]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const target = btn.getAttribute('data-copy');
        const node = target ? document.querySelector(target) : null;
        if (!node) return;
        const text = node.textContent.trim();
        try {
          await navigator.clipboard.writeText(text);
          showCopied(btn);
        } catch (_) {
          // fallback: select + execCommand
          const range = document.createRange();
          range.selectNodeContents(node);
          const sel = window.getSelection();
          sel.removeAllRanges();
          sel.addRange(range);
          try {
            document.execCommand('copy');
            showCopied(btn);
          } catch (__) {
            /* give up silently */
          }
          sel.removeAllRanges();
        }
      });
    });
  }

  function showCopied(btn) {
    const label = btn.querySelector('[data-copy-label]');
    const original = label ? label.textContent : '';
    btn.classList.add('copied');
    if (label) label.textContent = 'copied';
    setTimeout(() => {
      btn.classList.remove('copied');
      if (label) label.textContent = original;
    }, 2000);
  }

  // ---------- local clock ----------
  function initClock() {
    const clockEl = document.querySelector('[data-clock]');
    if (!clockEl) return;
    const tick = () => {
      const now = new Date();
      const hh = String(now.getHours()).padStart(2, '0');
      const mm = String(now.getMinutes()).padStart(2, '0');
      const ss = String(now.getSeconds()).padStart(2, '0');
      clockEl.textContent = `${hh}:${mm}:${ss}`;
    };
    tick();
    setInterval(tick, 1000);
  }

  // ---------- boot ----------
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      initLang();
      initCopyButtons();
      initClock();
    });
  } else {
    initLang();
    initCopyButtons();
    initClock();
  }
})();
