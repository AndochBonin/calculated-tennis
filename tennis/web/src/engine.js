// Calls into the Go engine running in public/worker.js. Calls run one at a time
// in the worker, off the main thread.
const worker = new Worker(`${import.meta.env.BASE_URL}worker.js`);
const pending = new Map();
let next = 0;

worker.onmessage = ({ data: { id, json } }) => {
  const { resolve, reject } = pending.get(id);
  pending.delete(id);
  const body = JSON.parse(json);
  if (body.error) reject(new Error(body.error));
  else resolve(body);
};

export function call(fn, ...args) {
  return new Promise((resolve, reject) => {
    pending.set(++next, { resolve, reject });
    worker.postMessage({ id: next, fn, args });
  });
}
