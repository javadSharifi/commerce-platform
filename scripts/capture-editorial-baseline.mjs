import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import http from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { acceptance, clock, emptyCover, hostA, hostB, otherTitle, publicApi, routes } from './editorial-baseline-fixture.mjs';

const [referenceDir, assetsDir, outputDir] = process.argv.slice(2);
const chrome = process.env.CHROME_BIN;
assert.ok(chrome && referenceDir && assetsDir && outputDir,
  'Usage: CHROME_BIN=path node scripts/capture-editorial-baseline.mjs REFERENCE_DIR ASSETS_DIR NEW_OUTPUT_DIR');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const tailwind = await readFile(join(assetsDir, 'tailwind.js'));
const font = await readFile(join(assetsDir, 'vazirmatn.woff2'));
assert.equal(hash(tailwind), 'a789ce5a73191759006b64a0c05f63afbf9aa43a86511bf798d688737429e60a', 'Stitch Tailwind bundle changed');
assert.equal(hash(font), '4e3fa217d38fdafc1fea4414ceb58ca5e662cf0ab5fa735a8c8c20e8b42cad92', 'Vazirmatn font changed');

const references = await Promise.all(routes.map(async (_, i) => readFile(join(referenceDir, `_${i + 1}`, 'code.html'), 'utf8')));
const referencesWithEmptyFrames = references.map(html => html
  .replace('https://cdn.tailwindcss.com?plugins=forms,container-queries', '/_baseline/tailwind.js')
  .replace(/<link\b[^>]*fonts\.googleapis\.com[^>]*>/g, '')
  .replace(/(<img\b[^>]*\bsrc=")https:\/\/lh3\.googleusercontent\.com\/[^"]+/g, `$1${emptyCover}`));
const assets = new Map([
  ['/_baseline/tailwind.js', ['text/javascript', tailwind]],
  ['/_baseline/vazirmatn.woff2', ['font/woff2', font]],
  [emptyCover, ['image/svg+xml', '<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="900"/>']],
  ...referencesWithEmptyFrames.map((html, i) => [`/_baseline/${i + 1}.html`, ['text/html; charset=utf-8', html]]),
]);

let astroAddress;
const server = http.createServer((req, res) => {
  const incomingHost = req.headers.host?.split(':')[0];
  const path = new URL(req.url, 'http://localhost').pathname;
  const host = (incomingHost === '127.0.0.1' && path.startsWith('/v1/')
    ? req.headers['x-forwarded-host'] : req.headers.host)?.split(':')[0];
  if (!publicApi.has(host)) return res.writeHead(404).end();
  if (path.startsWith('/v1/')) {
    const body = publicApi.get(host).get(path);
    const status = path === '/v1/account/auth/me' ? 401 : body === undefined ? 404 : 200;
    return res.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' })
      .end(JSON.stringify(body ?? { code: status === 401 ? 'UNAUTHORIZED' : 'NOT_FOUND' }));
  }
  const asset = assets.get(path);
  if (asset) return res.writeHead(200, { 'Content-Type': asset[0], 'Cache-Control': 'no-store' }).end(asset[1]);
  const upstream = http.request(`http://${astroAddress}${req.url}`, { headers: { Host: req.headers.host } }, response => {
    res.writeHead(response.statusCode, response.headers);
    response.pipe(res);
  });
  upstream.on('error', err => {
    console.error('Astro fixture upstream', err.message);
    if (!res.headersSent) res.writeHead(502);
    res.end();
  });
  res.on('close', () => upstream.destroy());
  upstream.end();
});

function freezeClock(instant) {
  const RealDate = Date;
  globalThis.Date = class extends RealDate {
    constructor(...args) { super(...(args.length ? args : [instant])); }
    static now() { return RealDate.parse(instant); }
  };
}

function preserveFixtureHost() {
  const nativeFetch = globalThis.fetch;
  globalThis.fetch = (url, options) => {
    const headers = new Headers(options?.headers);
    // Node replaces Host with the loopback origin. Carry it over the fixture hop.
    if (String(url).startsWith(process.env.INTERNAL_API_URL + '/') && headers.has('host')) {
      headers.set('x-forwarded-host', headers.get('host'));
    }
    return nativeFetch(url, { ...options, headers });
  };
}

const processes = [];
function start(command, args, options = {}) {
  const child = spawn(command, args, { ...options, stdio: ['ignore', 'pipe', 'pipe'] });
  const state = { child, output: '', stopped: false };
  state.finished = once(child, 'close').catch(err => { state.output += err.message; })
    .finally(() => { state.stopped = true; });
  for (const stream of [child.stdout, child.stderr]) {
    stream.on('data', data => { state.output = (state.output + data).slice(-6000); });
  }
  processes.push(state);
  return state;
}

async function until(description, check, process) {
  for (let attempt = 0; attempt < 160 && !process?.stopped; attempt++) {
    const result = await check();
    if (result) return result;
    await delay(75);
  }
  throw new Error(`Could not verify ${description}\n${process?.output ?? ''}`);
}

const profile = await mkdtemp(join(tmpdir(), 'psychology-baseline-'));
let socket;
let sequence = 0;
function cdp(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timeout = setTimeout(() => {
      socket.removeEventListener('message', receive);
      reject(new Error(`Chrome did not reply to ${method}`));
    }, 15_000);
    function receive(message) {
      const reply = JSON.parse(message.data);
      if (reply.id !== id) return;
      socket.removeEventListener('message', receive);
      clearTimeout(timeout);
      if (reply.error) reject(new Error(`${method}: ${reply.error.message}`));
      else resolve(reply.result);
    }
    socket.addEventListener('message', receive);
    socket.send(JSON.stringify({ id, method, params }));
  });
}

async function evaluate(expression) {
  const response = await cdp('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  assert.ok(!response.exceptionDetails, response.result.description);
  return response.result.value;
}

async function measure(isReference, selectInPerson, emptyCoverPath) {
  const main = document.querySelector('main');
  const heading = main.querySelector('h1');
  const root = getComputedStyle(document.documentElement);
  const nativeFontFaces = [...document.fonts].filter(face => face.family.includes('Vazirmatn')).length;
  const face = new FontFace('Vazirmatn', 'url(/_baseline/vazirmatn.woff2)', { weight: '100 900' });
  document.fonts.add(await face.load());
  await document.fonts.ready;
  if (selectInPerson) {
    const inPerson = document.querySelector('[data-format-option^="مشاوره حضوری"]');
    inPerson.click();
    if (inPerson.getAttribute('aria-pressed') !== 'true') throw new Error('In-person booking was not selected');
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    await Promise.all(inPerson.parentElement.getAnimations({ subtree: true }).map(animation => animation.finished));
  }
  const images = [...main.querySelectorAll('img')];
  await Promise.all(images.map(img => {
    if (img.getAttribute('src') !== emptyCoverPath) throw new Error('Expected a blank image frame');
    img.loading = 'eager';
    return img.decode();
  }));
  const rect = element => {
    if (!element) return null;
    const { x, y, width, height } = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    return {
      x, y, width, height, left: x, right: innerWidth - x - width,
      paddingInlineStart: style.paddingInlineStart, paddingInlineEnd: style.paddingInlineEnd,
      paddingBlockStart: style.paddingBlockStart,
    };
  };
  const box = main.querySelector(':scope > [class*="max-w-"]') ??
    main.querySelector('[class*="max-w-7xl"], [class*="max-w-5xl"]');
  const tint = selector => {
    const node = main.querySelector(selector);
    return node ? getComputedStyle(node).backgroundColor : null;
  };
  const { fontSize, fontWeight, lineHeight, fontFamily, color } = getComputedStyle(heading);
  return {
    viewport: [innerWidth, innerHeight, devicePixelRatio],
    direction: root.direction, theme: document.documentElement.dataset.theme ?? null,
    palette: document.documentElement.dataset.palette ?? null, nativeFontFaces, font: face.status,
    heading: { text: heading.textContent.trim(), fontSize, lineHeight, fontWeight, fontFamily, color, ...rect(heading) },
    header: rect(document.querySelector('body header')), shell: rect(main), content: rect(box),
    surface: getComputedStyle(document.querySelector('.editorial-shell') ?? document.body).backgroundColor,
    sections: [...main.querySelectorAll('h2')].map(node => node.textContent.trim()),
    card: tint(isReference ? 'article.bg-surface-card, .bg-surface-card:is(a, form, blockquote, div)' : '.card'),
    warm: tint(isReference ? '.bg-surface-warm' : '.bg-base-200'),
    primary: getComputedStyle(document.querySelector(isReference ? 'header .bg-primary' : 'header .btn-primary')).backgroundColor,
    tokens: Object.fromEntries(['base-100', 'base-200', 'base-300', 'base-content', 'primary', 'secondary', 'accent', 'surface-card']
      .map(key => [key, root.getPropertyValue(`--color-${key}`).trim()])),
    images: images.map(img => ({
      alt: img.alt, ...rect(img), objectFit: getComputedStyle(img).objectFit,
      frame: rect(img.parentElement), radius: getComputedStyle(img).borderRadius,
    })),
    portraitPlaceholder: rect(main.querySelector('[class~="aspect-[3/4]"]')),
  };
}

try {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const { port } = server.address();
  const clockCode = `(${freezeClock.toString()})(${JSON.stringify(clock)})`;
  const preload = `${clockCode};(${preserveFixtureHost.toString()})()`;
  const astro = start(process.execPath, [`--import=data:text/javascript,${encodeURIComponent(preload)}`, './dist/server/entry.mjs'], {
    cwd: new URL('../apps/psychology-site/', import.meta.url),
    env: { NODE_ENV: 'production', HOST: '127.0.0.1', PORT: '0', TZ: 'Asia/Tehran', INTERNAL_API_URL: `http://127.0.0.1:${port}` },
  });
  astroAddress = await until('the built Astro server', () => astro.output.match(/Server listening on (?:https?:\/\/)?(127\.0\.0\.1:\d+)/)?.[1], astro);
  const browser = start(chrome, [
    '--headless', '--remote-debugging-port=0', '--no-first-run', '--disable-background-networking', '--no-proxy-server',
    `--user-data-dir=${profile}`, `--host-resolver-rules=MAP ${hostA} 127.0.0.1, MAP ${hostB} 127.0.0.1, MAP * ~NOTFOUND`,
    'about:blank',
  ]);
  const debugPort = await until('the Chrome DevTools port', async () => {
    try { return (await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]; }
    catch (err) { if (err.code !== 'ENOENT') throw err; return null; }
  }, browser);
  const debuggerUrl = `http://127.0.0.1:${debugPort}`;
  const targets = await (await fetch(`${debuggerUrl}/json/list`)).json();
  socket = new WebSocket(targets.find(target => target.type === 'page').webSocketDebuggerUrl);
  await once(socket, 'open');
  await cdp('Page.enable');
  await cdp('Page.addScriptToEvaluateOnNewDocument', { source: clockCode });
  await cdp('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false });
  await cdp('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: 'light' }] });
  await cdp('Emulation.setTimezoneOverride', { timezoneId: 'Asia/Tehran' });

  const request = (host, path) => new Promise((resolve, reject) => {
    http.get({ hostname: '127.0.0.1', port, path, headers: { Host: host } }, resolve).on('error', reject);
  });
  const html = async response => Buffer.concat(await Array.fromAsync(response)).toString();
  const other = await request(hostB, '/');
  assert.equal(other.statusCode, 200);
  const otherHtml = await html(other);
  assert.ok(otherHtml.includes(otherTitle) && otherHtml.includes('data-palette="warm-sand"'));
  assert.ok(!otherHtml.includes(routes[5][1]));
  for (const [host, path] of [[hostB, routes[1][0]], [hostA, '/blog/no-such-article'], ['unknown.test', '/']]) {
    const missing = await request(host, path);
    assert.equal(missing.statusCode, 404);
    missing.resume();
  }
  await mkdir(outputDir);

  async function capture(path, filename, isReference, selectInPerson = false) {
    const url = `http://${hostA}:${port}${path}`;
    await cdp('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 0, y: 899 });
    const navigation = await cdp('Page.navigate', { url });
    assert.equal(navigation.errorText, undefined);
    await until(url, () => evaluate(`location.href === ${JSON.stringify(url)} && document.readyState === 'complete'`), browser);
    const measurement = await evaluate(`(${measure.toString()})(${isReference}, ${selectInPerson}, ${JSON.stringify(emptyCover)})`);
    assert.deepEqual(measurement.viewport, [1440, 900, 1]);
    assert.equal(measurement.direction, 'rtl');
    assert.equal(measurement.font, 'loaded');
    assert.match(measurement.heading.fontFamily, /Vazirmatn/);
    if (isReference) assert.equal(measurement.heading.fontSize, '44px', 'Stitch stylesheet must be active');
    else {
      assert.equal(measurement.theme, 'editorial');
      assert.equal(measurement.palette, acceptance ? 'stitch-editorial' : 'calm-sage');
      if (acceptance) {
        assert.equal(measurement.surface, 'rgb(250, 249, 246)');
        assert.equal(measurement.tokens.primary, '#2d4f43');
        assert.equal(measurement.primary, path === '/booking' ? 'rgb(34, 61, 52)' : 'rgb(45, 79, 67)');
        assert.equal(measurement.card, 'rgb(255, 255, 255)');
        assert.equal(measurement.tokens['base-200'], '#f3efe9');
        assert.equal(measurement.tokens.accent, '#c9825b');
        assert.ok(measurement.nativeFontFaces > 0, 'Astro must ship a local Vazirmatn font');
      }
    }
    const { cssContentSize } = await cdp('Page.getLayoutMetrics');
    assert.equal(cssContentSize.width, 1440);
    const { data } = await cdp('Page.captureScreenshot', {
      format: 'png', captureBeyondViewport: true,
      clip: { x: 0, y: 0, width: 1440, height: cssContentSize.height, scale: 1 },
    });
    await writeFile(join(outputDir, filename), Buffer.from(data, 'base64'));
    return measurement;
  }

  const pages = [];
  for (const [i, [path, expectedText, expectedFrames]] of routes.entries()) {
    const page = i + 1;
    const response = await request(hostA, path);
    assert.equal(response.statusCode, 200, path);
    assert.ok((await html(response)).includes(expectedText), `Missing fixture text at ${path}`);
    const reference = await capture(`/_baseline/${page}.html`, `_${page}-stitch.png`, true, path === '/booking');
    const ssr = await capture(path, `_${page}-ssr.png`, false);
    assert.equal(ssr.images.length, acceptance ? 0 : expectedFrames, `Unexpected number of image frames at ${path}`);
    pages.push({ path, referenceSource: `_${page}/code.html`, referenceHash: hash(references[i]), reference, ssr });
    console.log(`_${page} ${path} captured`);
  }
  const preview = await request(hostA, '/preview');
  assert.equal(preview.statusCode, 200);
  assert.equal(preview.headers['cache-control'], 'private, no-store');
  assert.ok((await html(preview)).includes(routes[5][1]));
  if (acceptance) {
    const cases = [
      [hostA, '/courses?topic=' + encodeURIComponent('اضطراب'), routes[2][1], 'هنر مرزبندی سالم در روابط'],
      [hostA, '/blog?search=' + encodeURIComponent('مرزهای سالم'), routes[1][1], 'بدن ما پیش از ذهن'],
      [hostB, '/courses', 'هنوز آموزشی منتشر نشده است', routes[2][1]],
      [hostB, '/blog', 'هنوز مقاله‌ای منتشر نشده است', routes[1][1]],
      [hostB, '/booking', 'درمانگری برای رزرو حضوری معرفی نشده است', 'محمد جواد شریفی'],
    ];
    for (const [host, path, present, absent] of cases) {
      const response = await request(host, path);
      const body = await html(response);
      assert.equal(response.statusCode, 200, path);
      assert.ok(body.includes(present) && !body.includes(absent), path);
      if (path.includes('?')) assert.match(response.headers['cache-control'], /no-store/);
    }
    const alternate = await request(hostA, '/preview?template=editorial&palette=ocean-trust');
    assert.equal(alternate.headers['cache-control'], 'private, no-store');
    assert.match(await html(alternate), /data-palette="ocean-trust"/);
    console.log('Two-host SSR, real filters, empty states, and alternate preview tested.');
  }
  const { Browser: browserVersion } = await (await fetch(`${debuggerUrl}/json/version`)).json();
  await writeFile(join(outputDir, 'measurements.json'), JSON.stringify({
    clock, browserVersion, host: hostA, fontHash: hash(font), tailwindHash: hash(tailwind), pages,
  }, null, 2) + '\n');
} finally {
  socket?.close();
  server.close();
  server.closeAllConnections();
  for (const state of processes.reverse()) {
    if (state.stopped) continue;
    state.child.kill('SIGTERM');
    const force = setTimeout(() => state.child.kill('SIGKILL'), 2000);
    await state.finished;
    clearTimeout(force);
  }
  await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}
