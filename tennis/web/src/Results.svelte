<script>
  import { GREY, LEVELS, fmtInt, pct, mix, scale, strength, summarize } from './theme.js';
  import Court from './Court.svelte';

  let { view, form, config, result, error, t, alpha, ontosetup, onrerun } = $props();

  let canvas = $state();
  let sel = $state(null);
  let layout = null;

  const outcomes = $derived(result ? result.matches.map(summarize) : []);
  const fmtLabel = $derived(config.formats.find((f) => f.id === result?.format)?.label ?? config.formats[form.format].label);
  const simsNum = $derived(Number(String(form.sims).replace(/[,_\s]/g, '')));
  const statusWord = $derived({ loading: 'Running', results: 'Done', error: 'Error' }[view]);
  const statusDot = $derived({ loading: '#e0a400', results: t.winA, error: '#b3261e' }[view]);

  const rows = $derived(result && [0, 1].map((i) => {
    const wins = i === 0 ? result.winsA : result.sims - result.winsA;
    return { tag: i ? 'B' : 'A', name: i ? result.playerB : result.playerA, color: i ? t.winB : t.winA,
      wins, pct: pct(wins / result.sims), hold: pct(result.rates[i].hold), brk: pct(result.rates[i].brk) };
  }));

  const details = $derived([
    { k: 'Format', v: fmtLabel },
    { k: 'Surface', v: t.upper },
    { k: 'First server', v: 'Coin toss' },
    { k: 'Alpha', v: String(result && view === 'results' ? result.alpha : (alpha ?? '—')) },
    { k: 'Sims', v: view === 'results' && result ? fmtInt(result.sims) : (Number.isFinite(simsNum) ? fmtInt(simsNum) : form.sims) }
  ]);

  // Scoreline of the selected square.
  const m = $derived.by(() => {
    if (!result || sel === null || sel >= result.matches.length) return null;
    const match = result.matches[sel], o = outcomes[sel];
    const names = [result.playerA, result.playerB], cols = [t.winA, t.winB];
    return {
      no: '#' + fmtInt(sel + 1), winner: names[o.winner], swatch: mix(GREY, cols[o.winner], strength(o.share)),
      rows: [0, 1].map((pi) => ({
        name: names[pi], color: cols[pi], bold: o.winner === pi, serve: match.f === pi ? ' •' : '',
        sets: match.s.map((s) => {
          const w = s[0] > s[1] ? 0 : 1;
          return { g: s[pi], tb: s.length === 4 && w !== pi ? String(s[2 + pi]) : '', won: w === pi };
        })
      })),
      summary: `Games ${o.games[0]}–${o.games[1]} · winner took ${pct(o.share)} of games · • served first`
    };
  });

  function draw() {
    const c = canvas, out = outcomes;
    if (!c || !out.length || !c.parentNode) return;
    const box = c.parentNode.getBoundingClientRect();
    const W = box.width, H = box.height, N = out.length;
    if (W < 4 || H < 4) return;
    const dpr = window.devicePixelRatio || 1;
    c.width = Math.round(W * dpr); c.height = Math.round(H * dpr);
    const ctx = c.getContext('2d');
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    // keep a margin so the selection outline is never clipped at the edges
    const M = 4, GW = W - 2 * M, GH = H - 2 * M;
    let size = Math.sqrt((GW * GH) / N), cols, rows;
    for (let k = 0; k < 400; k++) {
      cols = Math.max(1, Math.floor(GW / size)); rows = Math.ceil(N / cols);
      if (rows * size <= GH) break;
      size *= 0.985;
    }
    if (size >= 3) size = Math.floor(size);
    cols = Math.max(1, Math.floor(GW / size));
    const gap = size >= 10 ? 2 : (size >= 4 ? 1 : 0);
    const ox = M + (GW - cols * size) / 2, oy = M;
    const palA = [], palB = [];
    for (let i = 0; i <= LEVELS; i++) { const sh = 0.5 + 0.5 * i / LEVELS; palA.push(mix(GREY, t.winA, strength(sh))); palB.push(mix(GREY, t.winB, strength(sh))); }
    layout = { ox, oy, size, cols, N };
    let last = '';
    for (let p = 0; p < N; p++) {
      const o = out[p];
      const lv = Math.max(0, Math.min(LEVELS, Math.round((o.share - 0.5) * 2 * LEVELS)));
      const col = o.winner === 0 ? palA[lv] : palB[lv];
      if (col !== last) { ctx.fillStyle = col; last = col; }
      ctx.fillRect(ox + (p % cols) * size, oy + Math.floor(p / cols) * size, size - gap, size - gap);
    }
    if (sel !== null && sel < N) {
      const sx = ox + (sel % cols) * size, sy = oy + Math.floor(sel / cols) * size, sw = Math.max(size - gap, 2);
      // thin outline that scales with the square: 1px on dense grids, up to 2px on large squares
      const lw = size >= 14 ? 2 : (size >= 6 ? 1.5 : 1);
      const pad = lw / 2 + (size >= 6 ? 0.5 : 0);
      ctx.lineWidth = lw; ctx.strokeStyle = '#000';
      ctx.strokeRect(sx - pad, sy - pad, sw + 2 * pad, sw + 2 * pad);
    }
  }

  $effect(() => {
    void outcomes, sel, t, canvas; // redraw when any of these change
    draw();
  });
  $effect(() => {
    if (!canvas) return;
    const ro = new ResizeObserver(draw);
    ro.observe(canvas.parentNode);
    return () => ro.disconnect();
  });
  $effect(() => { void result; sel = null; });

  function pickSquare(e) {
    const L = layout;
    if (!L) return;
    const rect = canvas.getBoundingClientRect();
    const col = Math.floor((e.clientX - rect.left - L.ox) / L.size), row = Math.floor((e.clientY - rect.top - L.oy) / L.size);
    if (col < 0 || col >= L.cols || row < 0) return;
    const i = row * L.cols + col;
    if (i < L.N) sel = i;
  }
  function gridKey(e) {
    const L = layout;
    if (!L) return;
    if (e.key === 'Escape') { sel = null; return; }
    const step = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -L.cols, ArrowDown: L.cols }[e.key];
    if (!step) return;
    e.preventDefault();
    sel = Math.max(0, Math.min(L.N - 1, (sel ?? -step) + step));
  }
</script>

<section class="panel" aria-labelledby="ct-proj-h" aria-live="polite">
  <div class="rail" aria-hidden="true"><span><span class="sq" style="width:.7em;height:.7em"></span>Projection</span></div>
  <div class="body">
    <div class="titlebar">
      <h1 id="ct-proj-h">Match projection</h1>
      <span class="mark" style="gap:.5em;font-size:.95em"><span class="sq" style:background={statusDot}></span>{statusWord} · Step 2 of 2</span>
    </div>
    <div class="res" style="flex:1 1 auto;min-height:0;display:grid;gap:2.6em">
      <div style="display:flex;flex-direction:column;min-height:0">
        {#if view === 'loading'}
          <div role="status" style="flex:1 1 auto;display:flex;flex-direction:column;justify-content:center;gap:1.2em">
            <span class="big">Simulating…</span>
            <span class="muted">{form.nameA} vs {form.nameB} · {t.upper} · {Number.isFinite(simsNum) ? fmtInt(simsNum) : form.sims} runs</span>
            <div aria-hidden="true" style="display:grid;grid-template-columns:repeat(30,minmax(0,1fr));gap:3px">
              {#each { length: 30 } as _, i}
                <span class="pulse" style="height:1.6em;animation-delay:{(i * 0.04).toFixed(2)}s" style:background={t.winA}></span>
              {/each}
            </div>
          </div>
        {:else if view === 'results' && result}
          <div style="flex:1 1 auto;min-height:0;display:flex;flex-direction:column;gap:1.1em">
            <div style="display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:1.6em">
              {#each rows as p}
                <div style="display:flex;align-items:flex-end;justify-content:space-between;gap:1em;min-width:0;padding-bottom:.8em;border-bottom:3px solid {p.color}">
                  <div style="display:flex;flex-direction:column;gap:.3em;min-width:0">
                    <span class="lbl"><span class="sq" style:background={p.color}></span>Player {p.tag}</span>
                    <span class="plain" style="font-size:1.4em;font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{p.name}</span>
                    <span class="muted" style="font-size:.85em;font-variant-numeric:tabular-nums">{fmtInt(p.wins)} wins · Hold {p.hold} · Break {p.brk}</span>
                  </div>
                  <span style="font-size:3.6em;font-weight:500;line-height:.9;letter-spacing:-.03em;font-variant-numeric:tabular-nums" style:color={p.color}>{p.pct}</span>
                </div>
              {/each}
            </div>
            <div style="flex:1 1 auto;min-height:10em;position:relative">
              <!-- svelte-ignore a11y_no_interactive_element_to_noninteractive_role -->
              <canvas bind:this={canvas} tabindex="0" role="application" aria-roledescription="match grid" onclick={pickSquare} onkeydown={gridKey}
                aria-label="{fmtInt(result.sims)} simulated matches, one square each: {rows[0].name} won {rows[0].pct}, {rows[1].name} won {rows[1].pct}. Colour runs from player A through grey (close) to player B. Click or use arrow keys to inspect a match."
                style="position:absolute;inset:0;width:100%;height:100%;display:block;cursor:pointer"></canvas>
            </div>
            <div class="muted" style="display:flex;flex-wrap:wrap;align-items:center;gap:.6em 2em;font-size:.85em">
              <span>1 square = 1 match ({fmtInt(result.sims)})</span>
              <span class="mark">
                <span class="mark" style="gap:.4em"><span class="sq" style:background={t.winA}></span>A won every game</span>
                <span aria-hidden="true" style="display:flex;gap:2px">{#each scale(t.winA, t.winB, 11) as w}<span style="width:.9em;height:.9em" style:background={w}></span>{/each}</span>
                <span class="mark" style="gap:.4em">B won every game<span class="sq" style:background={t.winB}></span></span>
              </span>
              <span>Grey = close match · Click a square to inspect it</span>
            </div>
          </div>
        {:else if view === 'error'}
          <div style="flex:1 1 auto;display:flex;flex-direction:column;justify-content:center;gap:1em">
            <span class="big" style="color:#b3261e">Something went wrong</span>
            <span class="plain" style="padding:.8em 1em;border:1px solid #e2e2e2;background:#fafafa;font-size:1.05em">{error}</span>
            <span class="muted plain">Go back to setup to fix the player names, then run again.</span>
          </div>
        {/if}
      </div>

      <div style="display:flex;flex-direction:column;gap:1em;min-height:0">
        <span class="lbl"><span class="sq"></span>{t.upper} court</span>
        <Court {t} overview />
        {#if m}
          <div style="display:flex;flex-direction:column;gap:.8em;padding:1em;border:2px solid #141414;background:#fff">
            <div style="display:flex;justify-content:space-between;align-items:baseline;gap:1em">
              <span class="lbl"><span class="sq" style:background={m.swatch}></span>Match {m.no}</span>
              <button type="button" class="opt" aria-label="Close match details" style="min-height:2.2em;padding:0 .7em;font-size:.8em" onclick={() => (sel = null)}>Close</button>
            </div>
            <span class="plain" style="font-size:1.25em;font-weight:500;line-height:1.2">{m.winner} wins</span>
            <table style="border-collapse:collapse;width:100%;font-variant-numeric:tabular-nums">
              <caption style="position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)">Set scores</caption>
              <tbody>
                {#each m.rows as row}
                  <tr style="border-top:1px solid #ececec">
                    <th scope="row" class="plain" style="text-align:left;padding:.45em .4em .45em 0;max-width:9em;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" style:font-weight={row.bold ? 700 : 400}>
                      <span class="sq" style="width:.6em;height:.6em;margin-right:.45em" style:background={row.color}></span>{row.name}{row.serve}
                    </th>
                    {#each row.sets as c}
                      <td style="padding:.45em .35em;text-align:center;font-size:1.15em" style:font-weight={c.won ? 700 : 400} style:color={c.won ? '#141414' : '#8a8a8a'}>{c.g}<sup style="font-size:.6em;font-weight:400">{c.tb}</sup></td>
                    {/each}
                  </tr>
                {/each}
              </tbody>
            </table>
            <span class="muted" style="font-size:.85em;font-variant-numeric:tabular-nums">{m.summary}</span>
          </div>
        {:else}
          <div class="mark muted plain" style="gap:.6em;padding:1em;border:1px dashed #b9b9b9;font-size:.9em">
            <span class="sq" style="background:#bdbdbd"></span>Click any square to see that match's scoreline here.
          </div>
        {/if}
        <dl style="margin:0;display:flex;flex-direction:column">
          {#each details as d}
            <div style="display:flex;justify-content:space-between;gap:1em;padding:.65em 0;border-bottom:1px solid #ececec">
              <dt class="muted" style="font-size:.9em">{d.k}</dt>
              <dd class="plain" style="margin:0;font-weight:600;text-align:right;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{d.v}</dd>
            </div>
          {/each}
        </dl>
      </div>
    </div>
    <div style="display:flex;align-items:center;justify-content:space-between;gap:1em;padding-top:1.2em;border-top:1px solid #e2e2e2">
      <button type="button" class="opt" style="min-height:3.4em;padding:0 1.4em;border-color:#141414;font-size:1em" onclick={ontosetup}>← Back to setup</button>
      <span class="muted plain" style="font-size:.85em;text-align:center">Hold/break rates: Tennis Abstract, form-adjusted for the surface.</span>
      <button type="button" class="run" style="min-height:3.4em;padding:0 1.6em;font-size:1em" disabled={view === 'loading' || !result} onclick={onrerun}>Run again</button>
    </div>
  </div>
</section>
