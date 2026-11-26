// Polymarket public read APIs, called straight from the browser (both send CORS *).
// Gamma (events, scores) sits behind a CDN that caches for minutes; CLOB prices are live.

const GAMMA = 'https://gamma-api.polymarket.com/events?tag_id=864&closed=false&limit=100';
const CLOB_PRICES = 'https://clob.polymarket.com/prices';

const parse = (s) => { try { return JSON.parse(s); } catch { return null; } };

// parseEvents keeps open ATP singles events with a moneyline market, live first, then by start.
export function parseEvents(events) {
  const out = [];
  for (const e of events) {
    if (!e.slug?.startsWith('atp-') || e.slug.startsWith('atp-doubles-') || e.ended) continue;
    const m = e.markets?.find((m) => m.sportsMarketType === 'moneyline');
    const players = parse(m?.outcomes), tokens = parse(m?.clobTokenIds);
    if (players?.length !== 2 || tokens?.length !== 2) continue;
    const colon = e.title.indexOf(':');
    out.push({
      slug: e.slug,
      tournament: colon > 0 ? e.title.slice(0, colon) : '',
      players, tokens,
      start: new Date(e.startTime),
      live: !!e.live,
      score: e.score || '',
      period: e.period || ''
    });
  }
  return out.sort((a, b) => (b.live - a.live) || (a.start - b.start));
}

export async function fetchMatches() {
  const all = [];
  for (let offset = 0; ; offset += 100) {
    const r = await fetch(`${GAMMA}&offset=${offset}`, { cache: 'no-store' });
    if (!r.ok) throw new Error(`Polymarket events: ${r.status}`);
    const page = await r.json();
    all.push(...page);
    if (page.length < 100) return parseEvents(all);
  }
}

// fetchPrices returns {token: {bid, ask}}; CLOB's BUY side is the best bid, SELL the best ask.
export async function fetchPrices(tokens) {
  const r = await fetch(CLOB_PRICES, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(tokens.flatMap((t) => [{ token_id: t, side: 'BUY' }, { token_id: t, side: 'SELL' }]))
  });
  if (!r.ok) throw new Error(`Polymarket prices: ${r.status}`);
  const body = await r.json();
  const out = {};
  for (const t of tokens) {
    const p = body[t] || {};
    out[t] = { bid: p.BUY ? Number(p.BUY) : null, ask: p.SELL ? Number(p.SELL) : null };
  }
  return out;
}
