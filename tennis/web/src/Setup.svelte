<script>
  import { THEMES } from './theme.js';
  import Court from './Court.svelte';

  let { form = $bindable(), config, alphaOf, onrun, onreset } = $props();

  const PRESETS = [1000, 10000, 100000];
  let active = $state(null); // 'A' | 'B' while that input wants suggestions
  let err = $state('');

  const simsNum = $derived(Number(String(form.sims).replace(/[,_\s]/g, '')));
  const lower = $derived(config.players.map((n) => n.toLowerCase()));

  function suggestions(field) {
    const q = (field === 'A' ? form.nameA : form.nameB).trim().toLowerCase();
    if (active !== field || q.length < 2 || lower.includes(q)) return [];
    const starts = [], contains = [];
    lower.forEach((l, i) => {
      if (l.startsWith(q) || l.includes(' ' + q)) starts.push(config.players[i]);
      else if (l.includes(q)) contains.push(config.players[i]);
    });
    return starts.concat(contains).slice(0, 5);
  }

  function submit() {
    if (!form.nameA.trim() || !form.nameB.trim()) return (err = 'Both player names are required');
    if (!Number.isInteger(simsNum) || simsNum <= 0) return (err = 'Sims must be a whole number > 0');
    if (simsNum > 100000) return (err = 'Sims must be 100,000 or fewer');
    active = null;
    onrun(simsNum);
  }
  const onkey = (e) => { if (e.key === 'Enter') { e.preventDefault(); submit(); } };
  const pick = (field, name) => { field === 'A' ? (form.nameA = name) : (form.nameB = name); active = null; err = ''; };
</script>

{#snippet player(field, placeholder, color)}
  {@const sug = suggestions(field)}
  <div style="position:relative" onfocusout={(e) => { if (!e.currentTarget.contains(e.relatedTarget)) active = null; }}>
    <label for="ct-{field}" class="mark" style="position:absolute;left:.9em;top:50%;transform:translateY(-50%);gap:.45em;font-weight:600;pointer-events:none">
      <span class="sq" style="width:.6em;height:.6em" style:background={color}></span>{field}
    </label>
    {#if field === 'A'}
      <input id="ct-A" class="field" style="padding-left:3.1em" autocomplete="off" {placeholder} bind:value={form.nameA}
        oninput={() => { active = 'A'; err = ''; }} onfocus={() => (active = 'A')} onkeydown={onkey} />
    {:else}
      <input id="ct-B" class="field" style="padding-left:3.1em" autocomplete="off" {placeholder} bind:value={form.nameB}
        oninput={() => { active = 'B'; err = ''; }} onfocus={() => (active = 'B')} onkeydown={onkey} />
    {/if}
    {#if sug.length}
      <div role="listbox" aria-label="Player {field} suggestions" style="position:absolute;top:100%;left:0;right:0;z-index:5;display:flex;flex-direction:column;background:#fff;border:1px solid #141414;box-shadow:0 .6em 1.6em rgba(0,0,0,.12)">
        {#each sug as name}
          <button type="button" class="sug plain" onmousedown={(e) => e.preventDefault()} onclick={() => pick(field, name)}>{name}</button>
        {/each}
      </div>
    {/if}
  </div>
{/snippet}

<section class="panel" aria-labelledby="ct-setup-h">
  <div class="rail" aria-hidden="true"><span><span class="sq" style="width:.7em;height:.7em"></span>Setup</span></div>
  <div class="body">
    <div class="titlebar">
      <h1 id="ct-setup-h">Match setup</h1>
      <span class="muted" style="font-size:.95em">Step 1 of 2</span>
    </div>
    <div class="cols" style="flex:1 1 auto;min-height:0;display:grid;gap:2.6em">
      <div style="display:flex;flex-direction:column;gap:.8em;min-height:0">
        <span id="ct-surf-l" class="lbl"><span class="sq"></span>Court surface</span>
        <div role="radiogroup" aria-labelledby="ct-surf-l" style="display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:.9em">
          {#each THEMES as s, i}
            <button type="button" role="radio" aria-checked={i === form.surface} class="tile" style:border-color={i === form.surface ? '#141414' : 'transparent'} onclick={() => (form.surface = i)}>
              <Court t={s} />
              <span style="display:flex;justify-content:space-between;align-items:center;gap:.4em;padding:.7em .8em .3em;font-size:1.05em">
                <span class="mark" style="gap:.45em;font-weight:600"><span class="sq" style="width:.6em;height:.6em" style:background={s.dot}></span>{s.upper}</span>
                <span class="muted plain" style="font-size:.9em">α {alphaOf(i)?.toFixed(1) ?? '—'}</span>
              </span>
              <span class="muted plain" style="padding:0 .8em .8em;font-size:.9em;line-height:1.4">{s.blurb}</span>
            </button>
          {/each}
        </div>
        <p class="muted plain" style="margin:.4em 0 0;font-size:.95em;line-height:1.5;max-width:40em">The surface sets alpha — how strongly serve dominance shapes each game — and repaints the background to match the court.</p>
      </div>

      <div style="display:flex;flex-direction:column;gap:1.5em;min-height:0">
        <div style="display:flex;flex-direction:column;gap:.6em">
          <span class="lbl"><span class="sq"></span>Players</span>
          {@render player('A', 'e.g. Jannik Sinner', THEMES[form.surface].winA)}
          {@render player('B', 'e.g. Carlos Alcaraz', THEMES[form.surface].winB)}
        </div>

        <div style="display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:1em">
          <div style="display:flex;flex-direction:column;gap:.6em;min-width:0">
            <span id="ct-fmt-l" class="lbl"><span class="sq"></span>Format</span>
            <div role="group" aria-labelledby="ct-fmt-l" style="display:grid;grid-template-columns:repeat(2,minmax(0,1fr));border:1px solid #c8c8c8;background:#fff">
              {#each config.formats as f, i}
                <button type="button" class="opt" class:on={i === form.format} aria-pressed={i === form.format} style="min-height:3.1em;padding:0 .6em;border:0;font-size:.95em" onclick={() => (form.format = i)}>{f.label}</button>
              {/each}
            </div>
          </div>
          <div style="display:flex;flex-direction:column;gap:.6em;min-width:0">
            <label for="ct-sims" class="lbl"><span class="sq"></span>Simulations</label>
            <input id="ct-sims" class="field" inputmode="numeric" placeholder="e.g. 10000" style="font-variant-numeric:tabular-nums" bind:value={form.sims} oninput={() => (err = '')} onkeydown={onkey} />
          </div>
        </div>
        <div style="display:flex;gap:.5em;margin-top:-.6em">
          {#each PRESETS as p}
            <button type="button" class="opt" class:on={simsNum === p} aria-pressed={simsNum === p} style="min-height:2.75em;min-width:4.2em;padding:0 .8em;font-size:.9em" onclick={() => { form.sims = String(p); err = ''; }}>{p / 1000}K</button>
          {/each}
        </div>

        <div style="margin-top:auto;display:flex;flex-direction:column;gap:.7em">
          {#if err}
            <p role="alert" class="mark" style="margin:0;gap:.5em;color:#b3261e;font-weight:600"><span class="sq" style="background:#b3261e"></span>{err}</p>
          {/if}
          <div style="display:flex;gap:.6em">
            <button type="button" class="run" style="flex:1 1 auto;min-height:3.8em;font-size:1.15em;display:flex;align-items:center;justify-content:space-between;padding:0 1.2em" onclick={submit}>
              <span>Run simulation</span><span style="font-weight:400;opacity:.7">Enter ↵</span>
            </button>
            <button type="button" class="opt" style="min-height:3.8em;padding:0 1.2em;font-size:.95em" onclick={() => { err = ''; onreset(); }}>Reset</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>

<style>
  .tile { padding: 0; border: 2px solid transparent; background: #fff; display: flex; flex-direction: column; text-align: left; }
  .tile:hover { background: #f4f4f4; }
  .sug { text-align: left; padding: .55em 1em; min-height: 2.6em; border: 0; background: transparent; font-size: 1em; }
  .sug:hover, .sug:focus-visible { background: #f0f0f0; }
</style>
