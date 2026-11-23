import http from 'node:http';

// Node fetch replaces Host with the connection origin. These tests connect to
// loopback while navigating to a clinic domain on the HTTP wire.
export function fetchOnHost(url, options = {}) {
  const headers = new Headers(options.headers);
  if (!headers.has('host')) return fetch(url, options);
  return new Promise((resolve, reject) => {
    const request = http.request(url, { method: options.method ?? 'GET', headers: Object.fromEntries(headers) }, async response => {
      try {
        const body = Buffer.concat(await Array.fromAsync(response));
        resolve(new Response(body.length ? body : null, { status: response.statusCode, headers: response.headers }));
      } catch (err) {
        reject(err);
      }
    });
    request.on('error', reject).end(options.body);
  });
}
