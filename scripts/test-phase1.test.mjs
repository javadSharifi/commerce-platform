import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { delimiter, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const runner = fileURLToPath(new URL('./test-db.mjs', import.meta.url));

test('timeout and interruption stop a process group even when its leader exits first', async t => {
  const { startFixtureProcess } = await import('./fixture-process.mjs');
  for (const reason of ['interrupt', 'timeout']) {
    await t.test(reason, async () => {
      const worker = startFixtureProcess(process.execPath, ['-e', `
        const { spawn } = require('node:child_process');
        const child = spawn(process.execPath, ['-e',
          'process.on("SIGTERM", () => {}); console.log(process.pid); setInterval(() => {}, 1000)'],
          { stdio: ['ignore', 'inherit', 'inherit'] });
        setInterval(() => {}, 1000);
      `], { stdio: ['ignore', 'pipe', 'inherit'] });
      const descendant = await new Promise((resolve, reject) => {
        worker.child.stdout.once('data', chunk => resolve(Number(chunk.toString().trim())));
        worker.closed.then(result => reject(new Error(`fixture exited early: ${JSON.stringify(result)}`)));
      });
      t.after(() => worker.stop());
      const signal = reason === 'timeout' ? AbortSignal.timeout(20) : AbortSignal.abort();
      await assert.rejects(worker.wait(signal), /abort|timeout/i);
      assert.throws(() => process.kill(worker.child.pid, 0), { code: 'ESRCH' });
      // Linux can briefly retain an orphan's terminated process-table entry.
      const ps = spawnSync('ps', ['-o', 'stat=', '-p', String(descendant)], { encoding: 'utf8' });
      assert.match(ps.stdout.trim(), /^(Z\S*|)$/);
    });
  }
});

test('the database lane checks and pins a local Docker socket before creating resources', async t => {
  const dir = await mkdtemp(join(tmpdir(), 'phase1-docker-endpoint-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const calls = join(dir, 'calls');
  await writeFile(join(dir, 'docker'), `#!/bin/sh
echo "$*" >> "$DOCKER_FIXTURE_TEST_CALLS"
if [ "$1" = context ]; then
  echo "$DOCKER_FIXTURE_TEST_ENDPOINT"
  exit 0
fi
exit 88
`, { mode: 0o700 });

  for (const scenario of [
    { name: 'explicit remote TCP', host: 'tcp://192.0.2.1:2376' },
    { name: 'ambiguous loopback TCP', host: 'tcp://127.0.0.1:2375' },
    { name: 'remote context overrides local environment endpoint', host: 'unix:///tmp/docker.sock', context: 'remote', endpoint: 'ssh://fixture.invalid' },
    { name: 'explicit local Unix socket', host: 'unix:///tmp/docker.sock', local: true },
    { name: 'local active context', endpoint: 'unix:///tmp/docker.sock', local: true },
  ]) {
    await t.test(scenario.name, async () => {
      await writeFile(calls, '');
      const env = { ...process.env, PATH: dir + delimiter + process.env.PATH, DOCKER_FIXTURE_TEST_CALLS: calls };
      delete env.DOCKER_CONTEXT;
      delete env.DOCKER_HOST;
      if (scenario.host) env.DOCKER_HOST = scenario.host;
      if (scenario.context) env.DOCKER_CONTEXT = scenario.context;
      if (scenario.endpoint) env.DOCKER_FIXTURE_TEST_ENDPOINT = scenario.endpoint;
      const result = spawnSync(process.execPath, [runner], { env, encoding: 'utf8', timeout: 5000 });
      assert.equal(result.status, 1, result.stderr);
      const commands = await readFile(calls, 'utf8');
      if (scenario.local) {
        assert.match(commands, /--host unix:\/\/\/tmp\/docker\.sock create /);
      } else {
        assert.match(result.stderr, /local Unix socket/);
        assert.doesNotMatch(commands, /(?:^| )(?:run|create|start|exec) /m);
      }
    });
  }
});
