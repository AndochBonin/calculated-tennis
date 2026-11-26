# Calculated Tennis

Tennis simulations and Polymarket gambling based on *Calculated Bets* by Steven Skiena and [Tennis Abstract](https://www.tennisabstract.com) by Jeff Sackmann.

## Tennis simulation web UI

The tennis match simulator is a web app: a Go server (`tennis/cmd/web`) serving a Svelte front end (`tennis/web`). Run `make run` from the repo root (or `make -C tennis web`), then open http://localhost:8080. The landing screen lists live and upcoming ATP singles from Polymarket (scores from Gamma, which can lag a few minutes; live moneyline prices from the CLOB, fetched straight from the browser). Click **Simulate** next to the ATP matches title for the simulator: pick a court surface, two players, a format and the number of sims; the server fetches form-adjusted hold/break rates from Tennis Abstract and runs the Monte Carlo projection. Each simulated match is drawn as one square — click one to see its scoreline. The chosen court surface sets the alpha (essentially the weight of serve dominance on the outcome) and the theme: **hard** (blue), **clay** (terracotta), or **grass** (green).

For front-end development, run `go run ./cmd/web` in `tennis/` and `npm run dev` in `tennis/web/` (Vite proxies `/api` to the Go server).
