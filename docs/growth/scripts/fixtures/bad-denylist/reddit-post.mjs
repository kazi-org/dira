// Bad fixture: proves check-drafts.mjs's deny-list scan catches an HTTP
// POST to a forum API host. Never run this file.
await fetch('https://oauth.reddit.com/api/submit', {
  method: 'POST',
  body: new URLSearchParams({ title: 'posted by an agent' }),
});
