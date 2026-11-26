// Run: node --test src/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseEvents } from './polymarket.js';

const ev = (slug, extra = {}) => ({
  slug, title: 'Florianopolis: A vs B', startTime: '2026-11-26T20:00:00Z',
  markets: [{ sportsMarketType: 'moneyline', outcomes: '["A","B"]', clobTokenIds: '["1","2"]' }],
  ...extra
});

test('parseEvents keeps open ATP singles, live first', () => {
  const got = parseEvents([
    ev('atp-up-2026-11-26'),
    ev('atp-doubles-x-2026-11-26'),
    ev('itf-x-2026-11-26'),
    ev('atp-done-2026-11-26', { ended: true }),
    ev('atp-nomoney-2026-11-26', { markets: [{ sportsMarketType: 'tennis_set_winner' }] }),
    ev('atp-live-2026-11-26', { live: true, score: '6-4, 2-1', period: 'S2', startTime: '2026-11-26T21:00:00Z' })
  ]);
  assert.deepEqual(got.map((m) => m.slug), ['atp-live-2026-11-26', 'atp-up-2026-11-26']);
  assert.equal(got[0].tournament, 'Florianopolis');
  assert.deepEqual(got[0].players, ['A', 'B']);
  assert.deepEqual(got[0].tokens, ['1', '2']);
  assert.equal(got[0].score, '6-4, 2-1');
});
