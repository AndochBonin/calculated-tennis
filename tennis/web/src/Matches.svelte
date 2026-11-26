<script>
  import { fetchMatches, fetchPrices } from './polymarket.js';
  import { clockStr, pct } from './theme.js';

  let { t, onsimulate } = $props();

  let matches = $state.raw([]);
  let loaded = $state(false);
  let listErr = $state('');
  let slug = $state(null);
  let prices = $state.raw(null);
  let priceErr = $state('');

  const sel = $derived(matches.find((m) => m.slug === slug));
  const live = $derived(matches.filter((m) => m.live));
  const upcoming = $derived(matches.filter((m) => !m.live));
  const tokensKey = $derived(sel?.tokens.join(','));

  // ponytail: polling; Gamma's CDN caches for minutes anyway. Sports websocket if scores need to be real-time.
  $effect(() => {
    const load = () => fetchMatches()
      .then((m) => { matches = m; listErr = ''; })
      .catch((e) => (listErr = e.message))
      .finally(() => (loaded = true));
    load();
    const id = setInterval(load, 60_000);
    return () => clearInterval(id);
  });

  $effect(() => {
    if (!tokensKey) return;
    const tokens = tokensKey.split(',');
    let stale = false;
    prices = null;
    priceErr = '';
    const load = () => fetchPrices(tokens)
      .then((p) => { if (!stale) { prices = p; priceErr = ''; } })
      .catch((e) => { if (!stale) priceErr = e.message; });
    load();
    const id = setInterval(load, 5_000);
    return () => { stale = true; clearInterval(id); };
  });

  const mid = (p) => (p?.bid != null && p?.ask != null ? (p.bid + p.ask) / 2 : null);
  const cents = (v) => (v == null ? '—' : (100 * v).toFixed(1) + '¢');
  const status = (m) => (m.live ? [m.score, m.period].filter(Boolean).join(' · ') : clockStr(m.start));
</script>

{#snippet row(m)}
  <button type="button" class="row" class:on={m.slug === slug} aria-pressed={m.slug === slug} onclick={() => (slug = m.slug)}>
    <span class="muted" style="font-size:.85em">{m.tournament}</span>
    <span class="plain" style="font-weight:600">{m.players[0]} <span class="muted" style="font-weight:400">vs</span> {m.players[1]}</span>
    <span class="plain" style="font-variant-numeric:tabular-nums;text-align:right" style:color={m.live ? t.court : '#555'}>{status(m)}</span>
  </button>
  {#if m.slug === slug}<div class="inline">{@render detail()}</div>{/if}
{/snippet}

{#snippet detail()}
  {#if sel}
    <div style="display:flex;flex-direction:column;gap:.4em">
      <span class="lbl" style="justify-content:space-between"><span class="mark" style="gap:.5em"><span class="sq"></span>{sel.tournament} · {sel.live ? 'Live' : 'Upcoming'}</span><span style="color:#1652f0;font-weight:600">Polymarket</span></span>
      <span class="plain" style="font-size:1.2em;font-variant-numeric:tabular-nums">{status(sel)}</span>
    </div>
    {#each sel.players as name, i}
      {@const p = prices?.[sel.tokens[i]]}
      <div class="price" style:border-left-color={i ? t.winB : t.winA}>
        <span class="plain" style="font-weight:600;font-size:1.15em">{name}</span>
        <span class="big" style="font-variant-numeric:tabular-nums">{mid(p) == null ? '—' : pct(mid(p))}</span>
        <span class="muted" style="font-variant-numeric:tabular-nums">Bid {cents(p?.bid)} · Ask {cents(p?.ask)}</span>
      </div>
    {/each}
    {#if priceErr}
      <p role="alert" class="mark" style="margin:0;color:#b3261e;font-weight:600"><span class="sq" style="background:#b3261e"></span>{priceErr}</p>
    {/if}
    <span class="muted" style="font-size:.85em">Moneyline midpoint · prices refresh every 5s · scores may lag a few minutes</span>
  {:else}
    <span class="muted">Select a match to see its prices</span>
  {/if}
{/snippet}

{#snippet list(title, items)}
  <div style="display:flex;flex-direction:column;gap:.5em">
    <span class="lbl"><span class="sq" class:pulse={title === 'Live'} style:background={title === 'Live' ? t.court : null}></span>{title} · {items.length}</span>
    {#each items as m (m.slug)}{@render row(m)}{:else}<span class="muted" style="font-size:.9em">None</span>{/each}
  </div>
{/snippet}

<section class="panel" aria-labelledby="ct-matches-h">
  <div class="rail" aria-hidden="true"><span><span class="sq" style="width:.7em;height:.7em"></span>Matches</span></div>
  <div class="body">
    <div class="titlebar">
      <h1 id="ct-matches-h">ATP matches</h1>
      <button type="button" class="opt mark" style="min-height:2.6em;padding:0 1em;font-size:.9em" onclick={() => onsimulate(sel?.players)}><span class="sq"></span>Simulate</button>
    </div>
    {#if listErr}
      <p role="alert" class="mark" style="margin:0;color:#b3261e;font-weight:600"><span class="sq" style="background:#b3261e"></span>{listErr}</p>
    {/if}
    <div class="res" style="flex:1 1 auto;min-height:0;display:grid;gap:2.6em">
      <div class="scroll" style="display:flex;flex-direction:column;gap:1.6em">
        {#if !loaded}
          <span class="muted pulse">Loading matches…</span>
        {:else}
          {@render list('Live', live)}
          {@render list('Upcoming', upcoming)}
        {/if}
      </div>

      <div class="side" style="display:flex;flex-direction:column;gap:1.4em;min-width:0">{@render detail()}</div>
    </div>
  </div>
</section>

<style>
  .scroll { overflow-y: auto; min-height: 0; }
  .row { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2.4fr) 15em; gap: 1em; align-items: center; text-align: left; padding: .7em .9em; border: 1px solid #e2e2e2; background: #fff; font-size: 1em; }
  .row:hover { background: #f4f4f4; }
  .row.on { border-color: #141414; }
  .price { display: flex; flex-direction: column; gap: .3em; padding: .8em 1.1em; border-left: .35em solid; background: #fff; }
  /* Narrow: .res is one column (app.css), so show prices under the selected row instead of below the list */
  .inline { display: none; }
  @media (max-width: 1000px) {
    .inline { display: flex; flex-direction: column; gap: 1em; padding: .4em 0 .8em; }
    .side { display: none !important; }
  }
  @media (max-width: 640px) {
    .row { grid-template-columns: minmax(0, 1fr); gap: .2em; }
  }
</style>
