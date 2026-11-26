// Surface themes, background mosaic and colour helpers (ported from the design prototype).

// Order matches the server's /api/config surfaces: hard, clay, grass.
export const THEMES = [
  { id: 'hard', blurb: 'Fast, true bounce. Big serving pays.', name: 'Hard', upper: 'HARD', court: '#3a6fc4', surround: '#3f9a7a', dot: '#3a6fc4',
    winA: '#2350b8', winB: '#127a58', split: 0.86, bias: 0.18,
    top: ['#eaf1ff', '#c9dcff', '#a3c4ff', '#7eabff', '#5b93ff', '#4a86f7'],
    bottom: ['#e3f8ef', '#bff0da', '#93e6c2', '#64d9a6', '#3fcd8f', '#22c27c'],
    accent: '#d8c6f2' },
  { id: 'clay', blurb: 'Slow, high bounce. More breaks of serve.', name: 'Clay', upper: 'CLAY', court: '#c9672f', surround: '#9b5328', dot: '#c9672f',
    winA: '#a8421a', winB: '#2c5f86', split: 0.48, bias: 0.08,
    top: ['#fff3e6', '#ffdcbc', '#ffc493', '#ffaa6b', '#fb9150', '#f57d3a'],
    bottom: ['#ffe9dc', '#ffc9a8', '#ffa878', '#fa8a52', '#f2733a', '#e8622a'],
    accent: '#e4eed6' },
  { id: 'grass', blurb: 'Quick, low bounce. Serve rules.', name: 'Grass', upper: 'GRASS', court: '#4f9a3f', surround: '#3a7a30', dot: '#4f9a3f',
    winA: '#2a7d1c', winB: '#2a62a8', split: 0.2, bias: 0.15,
    top: ['#f2f6f8', '#ddeef8', '#c3e2f5', '#a5d3f1', '#87c3ec', '#6fb5e8'],
    bottom: ['#f1f8ee', '#daf3d0', '#bdeba9', '#9be182', '#7ed662', '#67cc47'],
    accent: '#dcc2f0' }
];

export const COLS = 48, ROWS = 27;
export const GREY = '#bdbdbd', LEVELS = 24;
const MONTHS = ['JANUARY', 'FEBRUARY', 'MARCH', 'APRIL', 'MAY', 'JUNE', 'JULY', 'AUGUST', 'SEPTEMBER', 'OCTOBER', 'NOVEMBER', 'DECEMBER'];

export const fmtInt = (n) => n.toLocaleString('en-US');
export const pct = (v) => (100 * v).toFixed(1) + '%';
const pad = (n) => (n < 10 ? '0' : '') + n;
function ordinal(d) {
  const s = (d % 10 === 1 && d !== 11) ? 'ST' : (d % 10 === 2 && d !== 12) ? 'ND' : (d % 10 === 3 && d !== 13) ? 'RD' : 'TH';
  return pad(d) + s;
}
export function clockStr(d) {
  return ordinal(d.getDate()) + ' ' + MONTHS[d.getMonth()] + ' ' + String(d.getFullYear()).slice(2) + ' · ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
}

// Deterministic hash noise for the mosaic.
function hash(x, y, seed) {
  let h = (x * 374761393 + y * 668265263 + seed * 2147483647) | 0;
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}
const smooth = (t) => t * t * (3 - 2 * t);
function vnoise(x, y, seed) {
  const x0 = Math.floor(x), y0 = Math.floor(y), fx = smooth(x - x0), fy = smooth(y - y0);
  const a = hash(x0, y0, seed), b = hash(x0 + 1, y0, seed), c = hash(x0, y0 + 1, seed), d = hash(x0 + 1, y0 + 1, seed);
  return (a + (b - a) * fx) + ((c + (d - c) * fx) - (a + (b - a) * fx)) * fy;
}
const mosaicCache = {};
export function mosaic(i) {
  if (mosaicCache[i]) return mosaicCache[i];
  const th = THEMES[i], out = [], seed = 17 + i * 31;
  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      // vertical streaks: low frequency along y, higher across x
      const n = vnoise(x / 2.2, y / 6, seed) * 0.65 + vnoise(x / 5, y / 3, seed + 7) * 0.35;
      const band = y / (ROWS - 1) + (vnoise(x / 3, y / 8, seed + 3) - 0.5) * 0.7;
      const pal = band > th.split ? th.bottom : th.top;
      const shade = Math.max(0, Math.min(pal.length - 1, Math.floor((n * 1.35 - 0.12 + th.bias + (hash(x, y, seed + 9) - 0.5) * 0.35) * pal.length)));
      out.push(hash(x, y, seed + 21) < 0.025 ? th.accent : pal[shade]);
    }
  }
  return (mosaicCache[i] = out);
}

const hexToRgb = (h) => [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];
export function mix(a, b, t) {
  const x = hexToRgb(a), y = hexToRgb(b);
  return 'rgb(' + Math.round(x[0] + (y[0] - x[0]) * t) + ',' + Math.round(x[1] + (y[1] - x[1]) * t) + ',' + Math.round(x[2] + (y[2] - x[2]) * t) + ')';
}
// share of games won by the winner: 0.5 (even) -> grey, 1 (won every game) -> full colour
export const strength = (share) => Math.max(0, Math.min(1, (share - 0.5) / 0.5));
// A colour -> grey -> B colour
export function scale(colA, colB, n) {
  const o = [], h = (n - 1) / 2;
  for (let i = 0; i < n; i++) o.push(i < h ? mix(GREY, colA, (h - i) / h) : (i > h ? mix(GREY, colB, (i - h) / h) : GREY));
  return o;
}

// summarize turns a server match {f, s} into winner (0 = A, 1 = B), total
// games per player, and the winner's share of all games played.
export function summarize(m) {
  const games = [0, 0], sets = [0, 0];
  for (const s of m.s) {
    games[0] += s[0]; games[1] += s[1];
    sets[s[0] > s[1] ? 0 : 1]++;
  }
  const winner = sets[0] > sets[1] ? 0 : 1;
  return { winner, games, share: games[winner] / (games[0] + games[1]) };
}
