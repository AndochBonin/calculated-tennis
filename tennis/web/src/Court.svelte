<script>
  // Isometric court. `overview` adds both players and uses the larger side-panel framing.
  let { t, overview = false } = $props();
</script>

<svg
  viewBox={overview ? '0 0 220 140' : '0 0 200 110'}
  style={overview ? 'width:100%;height:auto;max-height:11em;display:block' : 'width:100%;height:auto;display:block'}
  role={overview ? 'img' : undefined}
  aria-label={overview ? `${t.name} court overview` : undefined}
  aria-hidden={overview ? undefined : 'true'}
>
  <rect width={overview ? 220 : 200} height={overview ? 140 : 110} fill={t.surround} />
  <g transform={overview ? 'matrix(1.0825,0.625,-1.0825,0.625,82.4,26.6)' : 'matrix(0.953,0.55,-0.953,0.55,75.7,16.8)'}>
    <rect x="-16" y="-10" width="127.08" height="63.88" fill={t.surround} />
    <rect x="0" y="0" width="95.08" height="43.88" fill={t.court} stroke="#fff" stroke-width="2" vector-effect="non-scaling-stroke" />
    <line x1="0" y1="5.48" x2="95.08" y2="5.48" stroke="#fff" stroke-width="1.6" vector-effect="non-scaling-stroke" />
    <line x1="0" y1="38.4" x2="95.08" y2="38.4" stroke="#fff" stroke-width="1.6" vector-effect="non-scaling-stroke" />
    <line x1="21.94" y1="5.48" x2="21.94" y2="38.4" stroke="#fff" stroke-width="1.6" vector-effect="non-scaling-stroke" />
    <line x1="73.14" y1="5.48" x2="73.14" y2="38.4" stroke="#fff" stroke-width="1.6" vector-effect="non-scaling-stroke" />
    <line x1="21.94" y1="21.94" x2="73.14" y2="21.94" stroke="#fff" stroke-width="1.6" vector-effect="non-scaling-stroke" />
    <line x1="47.54" y1="-3" x2="47.54" y2="46.88" stroke="#141414" stroke-opacity=".14" stroke-width="2.5" vector-effect="non-scaling-stroke" />
  </g>
  {#if overview}
    <ellipse cx="53" cy="39.5" rx="5" ry="1.9" fill="#000" fill-opacity=".22" />
    <circle cx="53" cy="34" r="4.6" fill={t.winA} stroke="#fff" stroke-width="1.6" />
    <polygon points="137.1,54.4 83.0,85.7 83.0,75.7 137.1,44.4" fill="#fff" fill-opacity=".28" />
    <line x1="137.1" y1="44.4" x2="83.0" y2="75.7" stroke="#fff" stroke-width="1.8" stroke-linecap="round" />
    <line x1="137.1" y1="54.4" x2="137.1" y2="43.8" stroke="#2b2b2b" stroke-width="1.2" stroke-linecap="round" />
    <line x1="83.0" y1="85.7" x2="83.0" y2="75.1" stroke="#2b2b2b" stroke-width="1.2" stroke-linecap="round" />
    <ellipse cx="151" cy="87.6" rx="3.7" ry="1.4" fill="#000" fill-opacity=".18" />
    <circle cx="150" cy="82" r="3.4" fill="#dcef3c" />
    <ellipse cx="167" cy="104.5" rx="5" ry="1.9" fill="#000" fill-opacity=".22" />
    <circle cx="167" cy="99" r="4.6" fill={t.winB} stroke="#fff" stroke-width="1.6" />
  {:else}
    <polygon points="123.9,41.3 76.2,68.8 76.2,60.8 123.9,33.3" fill="#fff" fill-opacity=".28" />
    <line x1="123.9" y1="33.3" x2="76.2" y2="60.8" stroke="#fff" stroke-width="1.8" stroke-linecap="round" />
    <line x1="123.9" y1="41.3" x2="123.9" y2="32.7" stroke="#2b2b2b" stroke-width="1.2" stroke-linecap="round" />
    <line x1="76.2" y1="68.8" x2="76.2" y2="60.2" stroke="#2b2b2b" stroke-width="1.2" stroke-linecap="round" />
    <ellipse cx="91.5" cy="45.4" rx="3.5" ry="1.3" fill="#000" fill-opacity=".18" />
    <circle cx="90.5" cy="40" r="3.2" fill="#dcef3c" />
  {/if}
</svg>
