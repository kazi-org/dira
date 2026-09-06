// Progressive enhancement: all content and navigation remain usable without JS.
document.documentElement.classList.add('js');
const button = document.querySelector('#menu-btn');
const menu = document.querySelector('#mobile-menu');
function closeMenu(returnFocus = false) {
  menu?.classList.remove('open');
  button?.setAttribute('aria-expanded', 'false');
  if (button) button.textContent = 'Menu';
  if (returnFocus) button?.focus();
}
button?.addEventListener('click', () => {
  const open = menu.classList.toggle('open');
  button.setAttribute('aria-expanded', String(open));
  button.textContent = open ? 'Close' : 'Menu';
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && menu?.classList.contains('open')) closeMenu(true);
});
menu?.querySelectorAll('a').forEach(link => link.addEventListener('click', () => closeMenu()));
document.querySelectorAll('.nav-links a').forEach(link => {
  if (!new URL(link.href).hash && new URL(link.href).pathname === location.pathname) link.setAttribute('aria-current', 'page');
});
const explanations = {
  decision: 'The record identifies the choice your work currently follows.',
  rejected: 'A rejected alternative carries a reason, so a later session can examine the argument that settled it.',
  revisit: 'A decision can change. Record what changed and explicitly replace the old decision.'
};
function showPart() {
  const part = new URLSearchParams(location.search).get('part') || 'rejected';
  const selected = Object.hasOwn(explanations, part) ? part : 'rejected';
  document.querySelectorAll('.record-controls button').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.part === selected)));
  document.querySelectorAll('#record-panel .spec-row').forEach(row => row.classList.toggle('active', row.dataset.part === selected));
  const output = document.querySelector('#record-explain');
  if (output) output.textContent = explanations[selected];
}
document.querySelectorAll('.record-controls button').forEach(b => b.addEventListener('click', () => {
  const url = new URL(location.href);
  url.searchParams.set('part', b.dataset.part);
  url.hash = 'the-record';
  history.pushState({}, '', url);
  showPart();
}));
window.addEventListener('popstate', showPart);
showPart();
const live = document.querySelector('#live-region');
document.querySelectorAll('[data-copy]').forEach(b => b.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(b.dataset.copy);
    b.textContent = 'Copied';
    if (live) live.textContent = 'Command copied to clipboard.';
    setTimeout(() => { b.textContent = 'Copy'; }, 2000);
  } catch {
    const text = b.parentElement.querySelector('pre, code');
    if (text) {
      const range = document.createRange();
      range.selectNodeContents(text);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    }
    b.textContent = 'Select & copy';
    if (live) live.textContent = 'Clipboard unavailable. The command is selected; copy it with your browser.';
  }
}));
