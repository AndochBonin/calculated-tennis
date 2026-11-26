# Calculated Tennis

Tennis match simulations based on *Calculated Bets* by Steven Skiena and [Tennis Abstract](https://www.tennisabstract.com) by Jeff Sackmann.

## Web app

The simulator runs entirely in the browser: a Svelte front end (`tennis/web`) with the Go simulation engine (`tennis/cmd/wasm`) compiled to WebAssembly and run in a web worker. There is no server.

- `make run`: dev server (Vite) with hot reload. Rerun `make -C tennis wasm` after changing Go code.
- `make build`: static site in `tennis/web/dist`. Host it anywhere (GitHub Pages, Netlify, S3).
- `make test`: Go and front-end tests.

The landing screen lists live and upcoming ATP singles from Polymarket (scores from Gamma, which can lag a few minutes; live moneyline prices from the CLOB). Click **Simulate** next to the ATP matches title for the simulator: pick a court surface, two players, a format and the number of sims. Each simulated match is drawn as one square — click one to see its scoreline. The chosen court surface sets the alpha (essentially the weight of serve dominance on the outcome) and the theme: **hard** (blue), **clay** (terracotta), or **grass** (green).

Player stats come from Tennis Abstract's `jsfrags/{Player}.js` files. The site sends no CORS headers, so the worker loads those files with `importScripts` (allowed cross-origin) and caches them through the browser's HTTP cache. The engine then computes form-adjusted hold/break rates and runs the Monte Carlo projection.
