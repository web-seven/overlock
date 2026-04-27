(() => {
  'use strict';
  const $  = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));

  // year stamp
  const yr = $('#year'); if (yr) yr.textContent = new Date().getFullYear();

  // sync sticky command from main
  const heroCmd = $('#installCmd');
  const stuckCmd = $('#stuckCmd');
  if (heroCmd && stuckCmd) stuckCmd.textContent = heroCmd.textContent.trim();

  // sticky install bar
  const stuck = $('#stuck');
  const stuckX = $('#stuckX');
  let dismissed = sessionStorage.getItem('overlock_stuck3_dismissed') === '1';
  const onScroll = () => {
    if (!stuck || dismissed) return;
    stuck.classList.toggle('is-visible', window.scrollY > window.innerHeight * 0.85);
  };
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();
  if (stuckX) stuckX.addEventListener('click', () => {
    stuck.classList.remove('is-visible');
    dismissed = true;
    sessionStorage.setItem('overlock_stuck3_dismissed', '1');
  });

  // copy buttons
  $$('.copy').forEach(btn => {
    btn.addEventListener('click', async () => {
      const sel = btn.dataset.copy;
      const target = sel ? $(sel) : null;
      if (!target) return;
      const text = (target.innerText || target.textContent || '').trim();
      try {
        await navigator.clipboard.writeText(text);
      } catch {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed'; ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); } catch {}
        document.body.removeChild(ta);
      }
      btn.classList.add('is-copied');
      clearTimeout(btn._t);
      btn._t = setTimeout(() => btn.classList.remove('is-copied'), 1700);
    });
  });

  // reveal
  const targets = $$([
    '.hud-lbl', '.hero__logo', '.hero__title', '.hero__lede', '.launch', '.hero__actions',
    '.tile', '.panel__title', '.panel__copy', '.panel__terminal',
    '.engines__h', '.eng', '.wgmap', '.reg__card',
    '.qs li', '.cmp', '.tele__cell',
    '.final__title', '.final__actions'
  ].join(','));
  targets.forEach(t => t.classList.add('reveal'));
  if ('IntersectionObserver' in window) {
    const io = new IntersectionObserver(entries => {
      entries.forEach(e => {
        if (e.isIntersecting) { e.target.classList.add('is-in'); io.unobserve(e.target); }
      });
    }, { rootMargin: '0px 0px -8% 0px', threshold: 0.05 });
    targets.forEach(t => io.observe(t));
  } else targets.forEach(t => t.classList.add('is-in'));

  // counters
  const animateCount = (el, to, dur = 1100) => {
    const start = performance.now();
    const tick = (now) => {
      const p = Math.min(1, (now - start) / dur);
      const eased = 1 - Math.pow(1 - p, 3);
      const v = Math.round(to * eased);
      el.textContent = String(v).padStart(String(to).length, '0');
      if (p < 1) requestAnimationFrame(tick);
      else el.textContent = String(to);
    };
    requestAnimationFrame(tick);
  };
  $$('[data-count]').forEach(el => {
    const target = parseInt(el.dataset.count, 10);
    if (Number.isNaN(target)) return;
    if ('IntersectionObserver' in window) {
      const io = new IntersectionObserver(entries => {
        entries.forEach(e => { if (e.isIntersecting) { animateCount(el, target); io.unobserve(el); } });
      }, { threshold: 0.4 });
      io.observe(el);
    } else animateCount(el, target);
  });
})();
