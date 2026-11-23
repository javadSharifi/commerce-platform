import { spawn } from 'node:child_process';
import { addAbortListener } from 'node:events';
import { setTimeout as delay } from 'node:timers/promises';

export function fixtureCancellation() {
  const controller = new AbortController();
  const interrupt = () => controller.abort(new Error('fixture interrupted by SIGINT'));
  const terminate = () => controller.abort(new Error('fixture interrupted by SIGTERM'));
  process.on('SIGINT', interrupt);
  process.on('SIGTERM', terminate);
  return {
    signal: controller.signal,
    dispose() {
      process.off('SIGINT', interrupt);
      process.off('SIGTERM', terminate);
    },
  };
}

// Each fixture child owns a Unix process group so a leader exit cannot hide its descendants.
export function startFixtureProcess(command, args, { stopGrace = 2000, ...options } = {}) {
  const child = spawn(command, args, { ...options, detached: true });
  const closed = new Promise(resolve => {
    child.once('error', error => resolve({ error }));
    child.once('close', (code, signal) => resolve({ code, signal }));
  });
  let stopping;

  function signalGroup(signal) {
    if (!child.pid) return false;
    try {
      process.kill(-child.pid, signal);
      return true;
    } catch (err) {
      if (err.code !== 'ESRCH') throw err;
      return false;
    }
  }

  function stop() {
    stopping ??= (async () => {
      if (signalGroup('SIGTERM')) {
        const deadline = Date.now() + stopGrace;
        while (Date.now() < deadline && signalGroup(0)) await delay(25);
        signalGroup('SIGKILL');
      }
      await closed;
    })();
    return stopping;
  }

  async function wait(signal) {
    let listener;
    try {
      signal.throwIfAborted();
      const aborted = new Promise((_, reject) => {
        listener = addAbortListener(signal, () => reject(signal.reason));
      });
      return await Promise.race([closed, aborted]);
    } finally {
      listener?.[Symbol.dispose]();
      await stop();
    }
  }

  return { child, closed, wait, stop };
}
