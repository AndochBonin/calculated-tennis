// Classic (not module) worker: importScripts can load Tennis Abstract's
// cross-origin player scripts without CORS. Runs the Go engine (tennis.wasm),
// which registers config/rates/simulate on self; each returns a JSON string.
importScripts('wasm_exec.js');

const go = new Go();
const ready = WebAssembly.instantiateStreaming(fetch('tennis.wasm'), go.importObject)
  .then(({ instance }) => { go.run(instance); });

onmessage = async ({ data: { id, fn, args } }) => {
  try {
    await ready;
    postMessage({ id, json: self[fn](...args) });
  } catch (e) {
    postMessage({ id, json: JSON.stringify({ error: String(e) }) });
  }
};
