<script>
  import { onMount } from 'svelte';
  import { THEMES, mosaic, clockStr } from './theme.js';
  import Setup from './Setup.svelte';
  import Results from './Results.svelte';
  import Matches from './Matches.svelte';

  let config = $state({ players: [], surfaces: [], formats: [{ id: 'atp', label: 'Best of 3' }, { id: 'gs-men', label: 'Best of 5' }] });
  let form = $state({ surface: 2, nameA: '', nameB: '', format: 0, sims: '10000' });
  let screen = $state('matches'); // matches | sim
  let view = $state('setup'); // setup | loading | results | error
  let result = $state.raw(null);
  let error = $state('');
  let now = $state(new Date());
  let req = 0; // ignore responses from runs the user has navigated away from

  const t = $derived(THEMES[form.surface]);
  const alphaOf = (i) => config.surfaces[i]?.alpha;

  onMount(() => {
    fetch('/api/config').then((r) => r.json()).then((c) => (config = c)).catch(() => {});
    const tick = setInterval(() => (now = new Date()), 30000);
    return () => clearInterval(tick);
  });

  async function run(sims) {
    const id = ++req;
    view = 'loading';
    const json = async (r) => {
      const body = await r.json();
      if (!r.ok) throw new Error(body.error || r.statusText);
      return body;
    };
    try {
      const surface = THEMES[form.surface].id;
      const [playerA, playerB] = [form.nameA.trim(), form.nameB.trim()];
      // GET so the browser HTTP cache (Cache-Control from the server) skips repeat scrapes.
      const rates = await Promise.all([playerA, playerB].map((p) =>
        fetch(`/api/rates?player=${encodeURIComponent(p)}&surface=${surface}`).then(json)));
      if (id !== req) return;
      const body = await fetch('/api/simulate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ playerA, playerB, surface, format: config.formats[form.format].id, sims, rates })
      }).then(json);
      if (id !== req) return;
      result = body;
      view = 'results';
    } catch (e) {
      if (id !== req) return;
      error = e.message;
      view = 'error';
    }
  }

  function toSetup() { req++; view = 'setup'; }
  function reset() {
    req++;
    form = { surface: form.surface, nameA: '', nameB: '', format: 0, sims: '10000' };
    result = null;
    view = 'setup';
  }
</script>

<div class="ct" style:--hl={t.court}>
  <div class="mosaic" aria-hidden="true">
    {#each mosaic(form.surface) as c}<span style:background-color={c}></span>{/each}
  </div>
  {#if form.surface === 0}
    <svg class="layer" aria-hidden="true" style="mix-blend-mode:multiply;opacity:.5">
      <filter id="ct-grain-hard"><feTurbulence type="fractalNoise" baseFrequency=".85" numOctaves="2" stitchTiles="stitch" /><feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 -1.4 1.25" /></filter>
      <rect width="100%" height="100%" filter="url(#ct-grain-hard)" />
    </svg>
  {:else if form.surface === 1}
    <svg class="layer" aria-hidden="true" style="opacity:.75">
      <filter id="ct-powder" x="0" y="0" width="100%" height="100%"><feTurbulence type="fractalNoise" baseFrequency=".55" numOctaves="3" seed="4" stitchTiles="stitch" /><feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 .97  0 0 0 0 .93  1.7 0 0 0 -.78" /><feGaussianBlur stdDeviation=".9" /></filter>
      <rect width="100%" height="100%" filter="url(#ct-powder)" />
    </svg>
    <svg class="layer" aria-hidden="true" style="mix-blend-mode:multiply;opacity:.22">
      <filter id="ct-grain-clay"><feTurbulence type="fractalNoise" baseFrequency="1.1" numOctaves="1" stitchTiles="stitch" /><feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 .9  0 0 0 0 .85  0 0 0 -1.2 1.1" /></filter>
      <rect width="100%" height="100%" filter="url(#ct-grain-clay)" />
    </svg>
  {:else}
    <svg class="layer" aria-hidden="true" style="mix-blend-mode:multiply;opacity:.4">
      <filter id="ct-blades"><feTurbulence type="fractalNoise" baseFrequency="1.1 .045" numOctaves="2" seed="9" stitchTiles="stitch" /><feColorMatrix type="matrix" values="0 0 0 0 .35  0 0 0 0 .55  0 0 0 0 .3  -1.8 0 0 0 1.15" /></filter>
      <rect width="100%" height="100%" filter="url(#ct-blades)" />
    </svg>
    <svg class="layer" aria-hidden="true" style="mix-blend-mode:multiply;opacity:.7">
      <filter id="ct-grain-grass"><feTurbulence type="fractalNoise" baseFrequency=".7" numOctaves="3" seed="2" stitchTiles="stitch" /><feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 -1.6 1.35" /></filter>
      <rect width="100%" height="100%" filter="url(#ct-grain-grass)" />
    </svg>
  {/if}

  <div class="frame">
    <header class="head">
      <span class="mark" style="font-weight:600"><span class="sq" style="width:.75em;height:.75em"></span>Calculated Tennis</span>
      <span class="mark head-mid"><span class="sq" style="width:.75em;height:.75em"></span>{screen === 'matches' ? 'Live and upcoming matches' : 'Monte Carlo match projection'}</span>
      <span class="mark"><span class="sq" style="width:.75em;height:.75em"></span>{clockStr(now)}</span>
    </header>

    <div style="display:flex;min-height:0">
      {#if screen === 'matches'}
        <Matches {t} onsimulate={() => (screen = 'sim')} />
      {:else if view === 'setup'}
        <Setup bind:form {config} {alphaOf} onrun={run} onreset={reset} onmatches={() => (screen = 'matches')} />
      {:else}
        <Results {view} {form} {config} {result} {error} {t} alpha={alphaOf(form.surface)} ontosetup={toSetup} onrerun={() => run(result.sims)} onmatches={() => (screen = 'matches')} />
      {/if}
    </div>

    <footer style="display:flex;flex-wrap:wrap;justify-content:space-between;gap:.6em 2em;font-size:.95em">
      <span class="mark"><span class="sq"></span>Based on Calculated Bets × Tennis Abstract</span>
      <span class="mark"><span class="sq"></span>Surface · {t.upper}</span>
    </footer>
  </div>
</div>
