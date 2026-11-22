import test from 'node:test';
import assert from 'node:assert';
import { spawn } from 'node:child_process';

test('psychology-site SSR renders Button component from @psychology/ui', async () => {
  const port = 4391;
  const child = spawn('node', ['./dist/server/entry.mjs'], {
    env: { ...process.env, PORT: String(port), HOST: '127.0.0.1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  try {
    // Wait for server to start listening
    await new Promise((resolve, reject) => {
      child.stdout.on('data', (data) => {
        if (data.toString().includes('Server listening')) {
          resolve();
        }
      });
      child.on('error', reject);
      setTimeout(() => reject(new Error('timeout waiting for server to start')), 5000);
    });

    const res = await fetch(`http://127.0.0.1:${port}/`);
    assert.strictEqual(res.status, 200);
    const html = await res.text();
    assert.match(html, /<button/);
    assert.match(html, /btn-primary/);
    assert.match(html, /رزرو نوبت مشاوره/);
  } finally {
    child.kill('SIGTERM');
  }
});
