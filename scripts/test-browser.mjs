import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import http from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { once } from 'node:events';
import { setTimeout as delay } from 'node:timers/promises';
import { fixtureCancellation, startFixtureProcess } from './fixture-process.mjs';

const [apiAddress, hostA, titleA, hostB, titleB] = process.argv.slice(2);
assert.match(apiAddress ?? '', /^127\.0\.0\.1:\d+$/);
for (const host of [hostA, hostB]) assert.match(host, /^[a-z0-9-]+\.test$/);
const adminHost = 'admin.phase1.test';
const chrome = process.env.CHROME_BIN;
assert.ok(chrome, 'CHROME_BIN must be the path to an installed Chrome or Chromium executable');

const cancellation = fixtureCancellation();
const signal = AbortSignal.any([cancellation.signal, AbortSignal.timeout(55_000)]);
const children = [];
const network = [];
const consoleEvents = [];
let proxy;
let socket;
let profile;

function start(command, args, options) {
  signal.throwIfAborted();
  const worker = startFixtureProcess(command, args, { ...options, stdio: ['ignore', 'pipe', 'pipe'] });
  const state = { ...worker, output: '', exit: null };
  worker.closed.then(({ error, code, signal }) => { state.exit = error?.message ?? code ?? signal; });
  for (const stream of [state.child.stdout, state.child.stderr]) {
    stream.on('data', chunk => { state.output = (state.output + chunk).slice(-8000); });
  }
  children.push(state);
  return state;
}

async function waitFor(description, condition, processState) {
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    signal.throwIfAborted();
    if (processState?.exit !== null && processState?.exit !== undefined) {
      throw new Error(`${description}: process exited (${processState.exit})\n${processState.output}`);
    }
    const result = await condition();
    if (result) return result;
    await delay(50, undefined, { signal });
  }
  throw new Error(`timed out waiting for ${description}\n${processState?.output ?? ''}`);
}

async function astro(app, api) {
  const state = start(process.execPath, ['./dist/server/entry.mjs'], {
    cwd: new URL(`../apps/${app}/`, import.meta.url),
    env: { NODE_ENV: 'production', HOST: '127.0.0.1', PORT: '0', INTERNAL_API_URL: 'http://' + api },
  });
  return waitFor(app, () => state.output.match(/Server listening on (?:https?:\/\/)?(127\.0\.0\.1:\d+)/)?.[1], state);
}

const pending = new Map();
signal.addEventListener('abort', () => {
  for (const call of pending.values()) {
    clearTimeout(call.timeout);
    call.reject(signal.reason);
  }
  pending.clear();
}, { once: true });
let sequence = 0;
function cdp(method, params = {}) {
  signal.throwIfAborted();
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timeout = setTimeout(() => {
      pending.delete(id);
      reject(new Error('Chrome did not reply to ' + method));
    }, 5000);
    pending.set(id, { resolve, reject, timeout });
    socket.send(JSON.stringify({ id, method, params }));
  });
}

async function evaluate(expression) {
  const result = await cdp('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.text);
  return result.result.value;
}

function forward(req, res, upstream, headers = req.headers) {
  const outgoing = http.request({
    hostname: '127.0.0.1', port: upstream.split(':')[1],
    path: req.url, method: req.method, headers,
  }, response => {
    res.writeHead(response.statusCode, response.headers);
    response.pipe(res);
  });
  outgoing.on('error', err => {
    console.error('fixture upstream:', err.message);
    if (!res.headersSent) res.writeHead(502);
    res.end();
  });
  res.on('close', () => outgoing.destroy());
  req.pipe(outgoing);
}

try {
  profile = await mkdtemp(join(tmpdir(), 'psychology-browser-'));
  signal.throwIfAborted();
  const [site, admin] = await Promise.all([
    astro('psychology-site', apiAddress),
    astro('psychology-admin', apiAddress),
  ]);
  proxy = http.createServer((req, res) => {
    const host = req.headers.host?.split(':')[0];
    if (![hostA, hostB, adminHost].includes(host)) {
      res.writeHead(404).end();
      return;
    }
    const upstream = req.url.startsWith('/v1/') ? apiAddress : host === adminHost ? admin : site;
    forward(req, res, upstream);
  });
  proxy.listen(0, '127.0.0.1');
  await once(proxy, 'listening', { signal });
  const port = proxy.address().port;
  const browser = start(chrome, [
    '--headless', '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0',
    '--no-first-run', '--disable-background-networking', '--no-proxy-server',
    `--user-data-dir=${profile}`,
    `--host-resolver-rules=MAP ${hostA} 127.0.0.1, MAP ${hostB} 127.0.0.1, MAP ${adminHost} 127.0.0.1, MAP * ~NOTFOUND`,
    'about:blank',
  ]);
  const debugPort = await waitFor('Chrome DevTools port', async () => {
    try {
      return (await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).match(/^\d+/)?.[0];
    } catch (err) {
      if (err.code !== 'ENOENT') throw err;
      return undefined;
    }
  }, browser);
  const targets = await (await fetch(`http://127.0.0.1:${debugPort}/json/list`, {
    signal: AbortSignal.any([signal, AbortSignal.timeout(3000)]),
  })).json();
  socket = new WebSocket(targets.find(target => target.type === 'page').webSocketDebuggerUrl);
  await waitFor('Chrome debugger connection', () => socket.readyState === WebSocket.OPEN, browser);
  console.log('Phase 1 browser resources ' + JSON.stringify({ profile, groups: children.map(state => state.child.pid) }));
  const requests = new Map();
  socket.addEventListener('message', message => {
    const event = JSON.parse(message.data);
    const call = pending.get(event.id);
    if (call) {
      pending.delete(event.id);
      clearTimeout(call.timeout);
      if (event.error) call.reject(new Error(event.error.message));
      else call.resolve(event.result);
    }
    if (event.method === 'Network.requestWillBeSent') {
      requests.set(event.params.requestId, event.params.request.method);
    }
    if (event.method === 'Network.responseReceived') {
      const { requestId, response } = event.params;
      const cacheControl = Object.entries(response.headers).find(([name]) => name.toLowerCase() === 'cache-control')?.[1];
      network.push({ method: requests.get(requestId), url: response.url, status: response.status, cacheControl });
    }
    if (event.method === 'Runtime.consoleAPICalled') consoleEvents.push(event.params.type);
    if (event.method === 'Runtime.exceptionThrown') consoleEvents.push('exception');
  });
  await cdp('Page.enable');
  await cdp('Network.enable');
  await cdp('Runtime.enable');
  await cdp('Emulation.setDeviceMetricsOverride', {
    width: 1440, height: 900, deviceScaleFactor: 1, mobile: false,
  });
  await cdp('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: 'light' }] });

  async function navigate(host, path) {
    const destination = `http://${host}:${port}${path}`;
    const result = await cdp('Page.navigate', { url: destination });
    assert.equal(result.errorText, undefined, result.errorText);
    await waitFor(destination, () => evaluate(
      `location.href === ${JSON.stringify(destination)} && document.readyState === "complete"`,
    ), browser);
  }

  await navigate(hostA, '/');
  assert.equal(await evaluate('document.querySelector("h1")?.textContent.trim()'), titleA);
  assert.equal(await evaluate('document.documentElement.dataset.palette'), 'ocean-trust');
  const color = await evaluate('getComputedStyle(document.documentElement).getPropertyValue("--color-primary").trim()');
  assert.equal(color, '#0369a1', 'published ocean palette overrides the clinical template');
  assert.equal(await evaluate('window.innerWidth'), 1440);

  await navigate(hostB, '/');
  assert.equal(await evaluate('document.querySelector("h1")?.textContent.trim()'), titleB);
  assert.equal(await evaluate('document.documentElement.dataset.palette'), 'warm-sand');
  assert.equal(await evaluate('getComputedStyle(document.body).backgroundColor'), 'rgb(250, 246, 240)');

  await navigate(adminHost, '/login');
  const button = await evaluate(`(() => {
    document.getElementById('email').value = 'phase1-nonexistent@example.invalid';
    document.getElementById('password').value = ${JSON.stringify(randomUUID())};
    const rect = document.getElementById('login-submit-btn').getBoundingClientRect();
    return { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
  })()`);
  await cdp('Input.dispatchMouseEvent', { type: 'mousePressed', ...button, button: 'left', clickCount: 1 });
  await cdp('Input.dispatchMouseEvent', { type: 'mouseReleased', ...button, button: 'left', clickCount: 1 });
  const login = await waitFor('login response', () => network.find(event =>
    event.method === 'POST' && event.url.endsWith('/v1/admin/auth/login')), browser);
  assert.equal(login.status, process.env.PHASE1_BROWSER_FAIL_PROBE === '1' ? 200 : 401);
  const message = await waitFor('visible login feedback', () => evaluate(`(() => {
    const alert = document.getElementById('login-error-alert');
    return getComputedStyle(alert).display !== 'none' && alert.textContent.trim();
  })()`), browser);
  assert.equal(message, 'ایمیل یا رمز عبور اشتباه است.');
  assert.equal(await evaluate('location.pathname'), '/login');

  // Begin with a valid fixture session and selected clinic. Membership is still verified by Go.
  const { PSYCHOLOGY_FIXTURE_SESSION: sessionToken, PSYCHOLOGY_FIXTURE_TENANT: tenantID } = process.env;
  assert.ok(sessionToken && tenantID);
  const adminURL = `http://${adminHost}:${port}`;
  await cdp('Network.setCookies', { cookies: [{ name: 'session_token', value: sessionToken, url: adminURL, httpOnly: true }] });
  const discovery = await evaluate(`(async () => {
    const response = await fetch('/v1/admin/auth/me');
    const data = await response.json();
    const memberships = Array.isArray(data.memberships) ? data.memberships : [];
    return { status: response.status, count: memberships.length,
      containsClinic: memberships.some(membership => membership.tenant_id === ${JSON.stringify(tenantID)}) };
  })()`);
  console.log('Phase 1 browser clinic discovery ' + JSON.stringify(discovery));

  // The existing selected-clinic scenario verifies appearance independently of session discovery.
  await cdp('Network.setCookies', { cookies: [
    { name: 'tenant_id', value: tenantID, url: adminURL },
  ] });
  await navigate(adminHost, '/settings');
  assert.equal(await evaluate('document.getElementById("content-title").value'), titleA);
  const preview = new URL(await evaluate(`(() => {
    document.querySelector('input[name="template_id"][value="editorial"]').click();
    document.querySelector('input[name="palette_id"][value="stitch-editorial"]').click();
    return document.getElementById('preview-btn-top').href;
  })()`));
  assert.equal(preview.hostname, hostA);
  assert.equal(preview.pathname, '/preview');
  assert.equal(preview.searchParams.get('template'), 'editorial');
  assert.equal(preview.searchParams.get('palette'), 'stitch-editorial');

  async function readStitchColors() {
    return evaluate(`(() => {
      const root = getComputedStyle(document.documentElement);
      const background = selector => getComputedStyle(document.querySelector(selector)).backgroundColor;
      return {
        theme: document.documentElement.dataset.theme,
        surface: background('.editorial-shell'),
        card: background('main .card.bg-base-100'),
        warm: root.getPropertyValue('--color-base-200').trim(),
        primary: background('main .btn-primary'),
        accent: root.getPropertyValue('--color-accent').trim(),
      };
    })()`);
  }
  const stitch = {
    theme: 'editorial',
    surface: 'rgb(250, 249, 246)',
    card: 'rgb(255, 255, 255)',
    warm: '#f3efe9',
    primary: 'rgb(45, 79, 67)',
    accent: '#c9825b',
  };
  const previewPath = preview.pathname + preview.search;
  await navigate(hostA, previewPath);
  assert.deepEqual(await readStitchColors(), stitch);
  assert.equal(network.find(event => new URL(event.url).pathname === '/preview')?.cacheControl, 'private, no-store');
  await navigate(hostA, '/');
  assert.equal(await evaluate('document.documentElement.dataset.palette'), 'ocean-trust');

  await navigate(adminHost, '/settings');
  await evaluate(`(() => {
    document.querySelector('input[name="template_id"][value="editorial"]').click();
    document.querySelector('input[name="palette_id"][value="stitch-editorial"]').click();
    document.getElementById('save-appearance-btn').click();
  })()`);
  const save = await waitFor('panel appearance save', () => network.find(event =>
    event.method === 'PUT' && event.url.endsWith('/api/admin/appearance')), browser);
  assert.equal(save.status, 200);
  await waitFor('appearance success message', () => evaluate(
    '!document.getElementById("alert-appearance-success").classList.contains("hidden")',
  ), browser);
  await navigate(adminHost, '/settings');
  assert.equal(await evaluate('document.querySelector("input[name=palette_id]:checked").value'), 'stitch-editorial');

  const introduction = 'معرفی عمومی ثبت‌شده از پنل';
  await evaluate(`(() => {
    const field = document.getElementById('home-introduction');
    field.value = ${JSON.stringify(introduction)};
    field.dispatchEvent(new Event('input', { bubbles: true }));
    document.getElementById('save-content-btn').click();
  })()`);
  const profileSave = await waitFor('panel home profile save', () => network.find(event =>
    event.method === 'PUT' && event.url.endsWith('/api/admin/site')), browser);
  assert.equal(profileSave.status, 200);

  await navigate(hostA, '/');
  assert.equal(await evaluate('document.querySelector("h1").textContent.trim()'), titleA);
  assert.equal(await evaluate('document.documentElement.dataset.palette'), 'stitch-editorial');
  assert.deepEqual(await readStitchColors(), stitch);
  assert.ok(await evaluate(`document.querySelector('main').textContent.includes(${JSON.stringify(introduction)})`));
  const destinations = await evaluate(`[
    document.querySelector('main a[href="/courses/published-a"]')?.getAttribute('href'),
    document.querySelector('main a[href="/blog/published-a"]')?.getAttribute('href'),
    [...document.querySelectorAll('main a[href^="/workshops/"]')]
      .find(link => link.textContent.includes('published-a'))?.getAttribute('href'),
  ]`);
  assert.ok(destinations.every(Boolean), 'home cards link to published course, article and workshop details');
  const { name: clinicName, phone } = await evaluate('fetch("/v1/public/site").then(response => response.json())');
  const shell = await evaluate(`(() => {
    const header = document.querySelector('.editorial-shell > header');
    const footer = document.querySelector('.editorial-shell > footer');
    const main = document.querySelector('.editorial-shell > main');
    const section = main.querySelector(':scope > section');
    const bounds = element => {
      const { x, width } = element.getBoundingClientRect();
      return [x, width];
    };
    return {
      header: bounds(header.firstElementChild),
      headerHeight: header.getBoundingClientRect().height,
      footer: bounds(footer.firstElementChild),
      main: bounds(main),
      section: bounds(section),
      brand: header.querySelector('a[href="/"]').textContent.trim(),
      padding: getComputedStyle(section).paddingInlineStart,
      titleSize: getComputedStyle(main.querySelector('h1')).fontSize,
      footerSurface: getComputedStyle(footer).backgroundColor,
      primary: getComputedStyle(header.querySelector('a[href="/booking"].btn-primary')).backgroundColor,
    };
  })()`);
  assert.deepEqual(shell, {
    header: [80, 1280], headerHeight: 81, footer: [80, 1280], main: [0, 1440], section: [80, 1280],
    brand: clinicName, padding: '32px', titleSize: '44px', footerSurface: 'rgb(255, 255, 255)', primary: 'rgb(45, 79, 67)',
  });
  assert.ok(await evaluate('document.querySelector("footer").textContent.includes(' + JSON.stringify(phone) + ')'));
  await evaluate('document.fonts.ready.then(() => true)');
  await cdp('DOM.enable');
  await cdp('CSS.enable');
  const { root } = await cdp('DOM.getDocument');
  const { nodeId } = await cdp('DOM.querySelector', { nodeId: root.nodeId, selector: '.editorial-shell h1' });
  const { fonts } = await cdp('CSS.getPlatformFontsForNode', { nodeId });
  assert.ok(fonts.some(font => font.isCustomFont && /Vazirmatn/i.test(font.familyName)), 'heading uses bundled Vazirmatn');
  assert.ok(network.some(event => event.url.endsWith('.woff2') && event.status === 200));
  const tab = { key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 };
  await cdp('Input.dispatchKeyEvent', { ...tab, type: 'keyDown' });
  await cdp('Input.dispatchKeyEvent', { ...tab, type: 'keyUp' });
  assert.deepEqual(await evaluate(`(() => {
    const link = document.activeElement;
    const style = getComputedStyle(link);
    return [link.getAttribute('href'), link.matches(':focus-visible'), style.outlineStyle, style.outlineWidth, style.outlineColor];
  })()`), ['/', true, 'solid', '2px', 'rgb(45, 79, 67)']);

  for (const route of destinations) {
    await navigate(hostA, route);
    assert.equal(await evaluate('document.querySelector("main h1").textContent.trim()'), 'published-a');
  }
  await navigate(hostA, '/blog?empty=1');
  assert.equal(await evaluate('document.querySelector(".editorial-shell > header a[aria-current=page]").getAttribute("href")'), '/blog');
  assert.equal(await evaluate(`document.querySelector('.editorial-shell > header a[href="/"]').textContent.trim()`), clinicName);
  assert.equal(await evaluate('document.querySelector(".editorial-shell > main > div").getBoundingClientRect().width'), 1280);
  await navigate(hostA, '/preview?template=editorial&palette=ocean-trust');
  assert.equal(await evaluate('document.documentElement.dataset.theme'), 'editorial');
  assert.equal(await evaluate('document.querySelector(".editorial-shell > header a[aria-current=page]").getAttribute("href")'), '/');
  assert.equal(await evaluate('getComputedStyle(document.body).backgroundColor'), 'rgb(240, 247, 252)');
  await navigate(hostB, '/');
  assert.equal(await evaluate('document.querySelector("h1").textContent.trim()'), titleB);
  assert.ok(await evaluate(`!document.querySelector('main').textContent.includes(${JSON.stringify(introduction)}) &&
    document.querySelector('main a[href="/courses/published-a"]') === null`));
  assert.equal(await evaluate('document.documentElement.dataset.palette'), 'warm-sand');
  assert.equal(await evaluate('getComputedStyle(document.body).backgroundColor'), 'rgb(250, 246, 240)');
  assert.ok(!consoleEvents.includes('exception'), 'browser recorded no uncaught script exceptions');
  console.log('Home profile save, published destinations and two-host isolation passed');
  assert.equal(discovery.status, 200, 'a valid session must reach /v1/admin/auth/me');
  assert.equal(discovery.containsClinic, true, `authenticated clinic discovery omitted the seeded membership (${discovery.count} returned). See P2-T03.`);
  await cdp('Network.deleteCookies', { name: 'tenant_id', url: adminURL });
  await navigate(adminHost, '/settings');
  assert.equal(await evaluate('document.getElementById("content-title").value'), titleA);
  console.log(JSON.stringify({ hosts: [hostA, hostB], viewport: '1440x900 light', color, login, message, save, stitch, shell, fonts, consoleEvents }));
} finally {
  socket?.close();
  proxy?.close();
  proxy?.closeAllConnections();
  try {
    const results = await Promise.allSettled(children.map(state => state.stop()));
    const failures = results.filter(result => result.status === 'rejected');
    if (failures.length) {
      throw new AggregateError(failures.map(result => result.reason), 'failed to stop browser fixture processes');
    }
  } finally {
    try {
      if (profile) await rm(profile, { recursive: true, force: true });
    } finally {
      cancellation.dispose();
    }
  }
}
