import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { access, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { startFixtureProcess } from './fixture-process.mjs';

function launch(t, script, args, env) {
  const path = fileURLToPath(new URL(script, import.meta.url));
  const worker = startFixtureProcess(process.execPath, [path, ...args], { env, stopGrace: 12_000, stdio: ['ignore', 'pipe', 'pipe'] });
  t.after(() => worker.stop());
  let output = '';
  for (const stream of [worker.child.stdout, worker.child.stderr]) {
    stream.setEncoding('utf8');
    stream.on('data', chunk => { output = (output + chunk).slice(-100_000); });
  }
  return {
    ...worker,
    async observe(pattern) {
      const deadline = Date.now() + 30_000;
      while (Date.now() < deadline && worker.child.exitCode === null && worker.child.signalCode === null) {
        const match = output.match(pattern);
        if (match) return match;
        await delay(30);
      }
      assert.fail(`fixture did not report ${pattern}\n${output}`);
    },
    output: () => output,
  };
}

async function assertGone(pid, group = false) {
  const deadline = Date.now() + 3000;
  while (Date.now() < deadline) {
    try {
      process.kill(group ? -pid : pid, 0);
    } catch (err) {
      if (err.code === 'ESRCH') return;
      throw err;
    }
    await delay(25);
  }
  // An orphaned zombie has terminated, but the OS can retain its entry until reaping.
  const states = execFileSync('ps', ['-axo', 'pid=,pgid=,stat='], { encoding: 'utf8' }).trim().split('\n');
  const live = states.map(row => row.trim().split(/\s+/)).filter(([processID, groupID, status]) =>
    Number(group ? groupID : processID) === pid && !status.startsWith('Z'));
  assert.deepEqual(live, [], `fixture ${group ? 'group' : 'process'} ${pid} still running`);
}

test('interrupting the live database lane removes its container and terminates the API', { timeout: 50_000 }, async t => {
  const worker = launch(t, './test-db.mjs', [], { ...process.env, PSYCHOLOGY_TEST_BROWSER: '0' });
  const [, endpoint] = await worker.observe(/Phase 1 Docker endpoint (unix:\/\/\/[^\n]+)/);
  const [, workspace] = await worker.observe(/Phase 1 fixture workspace ([^\n]+)/);
  const [, container] = await worker.observe(/Phase 1 fixture container (psychology-test-[a-f0-9]{24})/);
  const env = { ...process.env };
  for (const key of ['DOCKER_CONTEXT', 'DOCKER_HOST', 'DOCKER_TLS', 'DOCKER_TLS_VERIFY']) delete env[key];
  const docker = args => execFileSync('docker', ['--host', endpoint, ...args], { env, encoding: 'utf8', timeout: 5000 }).trim();
  const list = () => docker(['ps', '-aq', '--filter', `name=^/${container}$`]);
  t.after(async () => {
    await worker.stop();
    if (list()) docker(['rm', '--force', '--volumes', container]);
    await rm(workspace, { recursive: true, force: true });
  });
  const [, apiPID] = await worker.observe(/API executable pid=(\d+)/);
  assert.match(list(), /^[a-f0-9]+$/);
  await access(workspace);
  worker.child.kill('SIGINT');
  const result = await worker.closed;
  assert.equal(result.code, 1, worker.output());
  assert.match(worker.output(), /fixture interrupted by SIGINT/);
  assert.equal(list(), '', 'the interrupted database container should be removed');
  await assertGone(Number(apiPID));
  await assert.rejects(access(workspace), { code: 'ENOENT' });
});

test('timing out the live browser lane removes Chrome, both SSR processes, and the profile', { timeout: 50_000 }, async t => {
  const chrome = process.env.CHROME_BIN;
  assert.ok(chrome, 'set CHROME_BIN to the Chrome executable after building the SSR apps');
  // Hold the local upstream response open until the browser supervisor is cancelled.
  const api = createServer((_req, res) => { res.writeHead(503); });
  api.listen(0, '127.0.0.1');
  t.after(() => { api.close(); api.closeAllConnections(); });
  await once(api, 'listening');
  const worker = launch(t, './test-browser.mjs', [
    `127.0.0.1:${api.address().port}`, 'a.test', 'Clinic A', 'b.test', 'Clinic B',
  ], { CHROME_BIN: chrome });
  const [, resources] = await worker.observe(/Phase 1 browser resources (\{[^\n]+\})/);
  const { profile, groups } = JSON.parse(resources);
  t.after(async () => {
    await worker.stop();
    for (const group of groups) {
      try {
        process.kill(-group, 'SIGKILL');
      } catch (err) {
        if (err.code !== 'ESRCH') throw err;
      }
    }
    await rm(profile, { recursive: true, force: true });
  });
  assert.equal(groups.length, 3);
  for (const group of groups) assert.doesNotThrow(() => process.kill(-group, 0));
  await access(profile);
  await assert.rejects(worker.wait(AbortSignal.timeout(25)), { name: 'TimeoutError' });
  assert.match(worker.output(), /fixture interrupted by SIGTERM/);
  for (const group of groups) await assertGone(group, true);
  const processes = execFileSync('ps', ['-axo', 'command='], { encoding: 'utf8', maxBuffer: 4 * 1024 * 1024 });
  assert.ok(!processes.includes(profile), 'no Chrome process should reference the fixture profile');
  await assert.rejects(access(profile), { code: 'ENOENT' });
});
