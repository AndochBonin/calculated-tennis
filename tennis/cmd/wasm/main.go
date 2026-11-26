//go:build js && wasm

package main

import (
	"errors"
	"syscall/js"
	"time"

	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func main() {
	g := js.Global()
	g.Set("config", js.FuncOf(func(js.Value, []js.Value) any { return toJSON(config(), nil) }))
	g.Set("rates", js.FuncOf(func(_ js.Value, a []js.Value) any { return toJSON(fetchRates(a[0].String())) }))
	g.Set("simulate", js.FuncOf(func(_ js.Value, a []js.Value) any { return toJSON(simulate(a[0].String())) }))
	select {}
}

// fetchRates loads the player's Tennis Abstract fragment with importScripts:
// the site sends no CORS headers, but a classic worker may import cross-origin
// scripts, and the browser HTTP cache honours Tennis Abstract's max-age.
// The worker runs one call at a time, so the shared player_frag global is safe.
func fetchRates(name string) (r rates, err error) {
	slug := tennisabstract.PlayerSlug(name)
	if slug == "" {
		return rates{}, errors.New("player name is required")
	}
	g := js.Global()
	g.Set("player_frag", js.Undefined())
	defer func() {
		if recover() != nil { // importScripts throws on 404 or network failure
			r, err = rates{}, noData(name)
		}
	}()
	g.Call("importScripts", "https://www.tennisabstract.com/jsfrags/"+slug+".js")
	frag := g.Get("player_frag")
	if frag.Type() != js.TypeString {
		return rates{}, noData(name)
	}
	return ratesFromHTML(name, frag.String(), time.Now())
}
