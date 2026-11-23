import { randomBytes } from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { fixtureCancellation, startFixtureProcess } from './fixture-process.mjs';

const api = fileURLToPath(new URL('../apps/psychology-api/', import.meta.url));
const id = randomBytes(12).toString('hex');
const database = 'psy_phase1_' + id;
const migrationDatabase = 'psy_phase1_' + randomBytes(12).toString('hex');
const container = 'psychology-test-' + id;
const owner = 'psychology.phase1.fixture=' + id;
const adminPassword = randomBytes(24).toString('hex');
const appPassword = randomBytes(24).toString('hex');
const workerPassword = randomBytes(24).toString('hex');
const dockerEnv = { ...process.env };
delete dockerEnv.DOCKER_HOST;
delete dockerEnv.DOCKER_CONTEXT;
delete dockerEnv.DOCKER_TLS;
delete dockerEnv.DOCKER_TLS_VERIFY;
const fixtureEnv = {
  ...dockerEnv,
  POSTGRES_USER: 'phase1_admin',
  POSTGRES_PASSWORD: adminPassword,
  POSTGRES_DB: database,
  PHASE1_APP_PASSWORD: appPassword,
  PHASE1_WORKER_PASSWORD: workerPassword,
};
const requiredTests = [
  'TestMigrationUpDownUpLive',
  'TestServer_LiveSettingsWithRealPool',
  'TestSiteSettings_LiveTenantIsolation',
  'TestOutbox_LiveTransactionalWorkflowAndPurge',
  'TestSiteSettings_PaletteIdMigrationAndSeedIsolation',
  'TestSiteSettings_SeparatedQueriesAndOutboxRollback',
  'TestAdminAppearance_LiveHttpGuardAndIsolation',
  'TestAcceptance_PSY_A12_LivePostgreSQL_TwoHosts',
  'TestAcceptance_PSY_A13_LivePostgreSQL_TwoHosts',
];

const cancellation = fixtureCancellation();

async function run(command, args, { env, cwd, input, signal, timeout = 30_000, stopGrace, onLine } = {}) {
  signal?.throwIfAborted();
  const deadline = AbortSignal.timeout(timeout);
  const worker = startFixtureProcess(command, args, {
    env, cwd, stopGrace, stdio: [input ? 'pipe' : 'ignore', 'pipe', 'inherit'],
  });
  let output = '';
  let streamError;
  if (onLine) {
    createInterface({ input: worker.child.stdout }).on('line', line => {
      try {
        onLine(line);
      } catch (error) {
        streamError ??= error;
      }
    });
  } else {
    worker.child.stdout.setEncoding('utf8');
    worker.child.stdout.on('data', chunk => { output += chunk; });
  }
  if (input) {
    worker.child.stdin.on('error', error => { streamError ??= error; });
    worker.child.stdin.end(input);
  }
  const result = await worker.wait(signal ? AbortSignal.any([signal, deadline]) : deadline);
  if (result.error || result.code !== 0) {
    throw new Error(`${command} ${args[0]} failed (${result.error?.code ?? result.code ?? result.signal})`);
  }
  if (streamError) throw streamError;
  return output.trim();
}

async function dockerEndpoint() {
  const context = process.env.DOCKER_CONTEXT;
  const endpoint = (!context && process.env.DOCKER_HOST) || await run('docker', [
    'context', 'inspect', ...(context ? [context] : []), '--format', '{{.Endpoints.docker.Host}}',
  ], { env: dockerEnv, signal: cancellation.signal });
  if (!/^unix:\/\/\/[^\r\n\0]+$/.test(endpoint)) {
    throw new Error('Phase 1 requires a local Unix socket for Docker. Select a local Docker context or set DOCKER_HOST to its Unix socket. Remote or TCP endpoints are unsupported.');
  }
  return endpoint;
}

function psql(docker, sql) {
  return docker([
    'exec', '-i', '-e', 'PHASE1_APP_PASSWORD', '-e', 'PHASE1_WORKER_PASSWORD', container,
    'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-v', 'fixture_name=' + database,
    '-v', 'migration_name=' + migrationDatabase,
    '-U', 'phase1_admin', '-d', database,
  ], { input: sql });
}

let scratch;
try {
  const endpoint = await dockerEndpoint();
  cancellation.signal.throwIfAborted();
  console.log(`Phase 1 Docker endpoint ${endpoint}`);
  scratch = await mkdtemp(join(tmpdir(), 'psychology-test-'));
  console.log(`Phase 1 fixture workspace ${scratch}`);
  const docker = (args, options) => run('docker', ['--host', endpoint, ...args], {
    env: fixtureEnv, signal: cancellation.signal, ...options,
  });
  try {
    cancellation.signal.throwIfAborted();
    // Let create finish on interrupt so cleanup can reconcile it by its unique ownership label.
    await docker([
      'create', '--pull=never', '--name', container, '--label', owner,
      '--tmpfs', '/var/lib/postgresql/data', '-p', '127.0.0.1::5432',
      '-e', 'POSTGRES_USER', '-e', 'POSTGRES_PASSWORD', '-e', 'POSTGRES_DB', 'postgres:16-alpine',
    ], { signal: undefined });
    console.log(`Phase 1 fixture container ${container}`);
    await docker(['start', container]);
    const deadline = Date.now() + 30_000;
    for (;;) {
      try {
        await docker(['exec', container, 'pg_isready', '-q', '-h', '127.0.0.1', '-U', 'phase1_admin', '-d', database], { timeout: 3000 });
        break;
      } catch (err) {
        cancellation.signal.throwIfAborted();
        if (Date.now() >= deadline) throw new Error('fixture PostgreSQL did not become ready', { cause: err });
        await delay(250, undefined, { signal: cancellation.signal });
      }
    }

    await psql(docker, String.raw`
\getenv app_password PHASE1_APP_PASSWORD
\getenv worker_password PHASE1_WORKER_PASSWORD
SELECT 'psychology-phase1:' || :'fixture_name' AS fixture_marker \gset
COMMENT ON DATABASE :"fixture_name" IS :'fixture_marker';
CREATE ROLE app_user LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD :'app_password';
CREATE ROLE worker_user LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD :'worker_password';
CREATE ROLE lookup_user NOLOGIN NOSUPERUSER NOBYPASSRLS;
CREATE DATABASE :"migration_name" TEMPLATE template0;
SELECT 'psychology-phase1:' || :'migration_name' AS migration_marker \gset
COMMENT ON DATABASE :"migration_name" IS :'migration_marker';
`);
    const migrations = new URL('../apps/psychology-api/migrations/', import.meta.url);
    const files = readdirSync(migrations).filter(name => name.endsWith('.up.sql')).sort();
    if (!files.length) throw new Error('no migrations found');
    for (const file of files) await psql(docker, readFileSync(new URL(file, migrations)));

    const address = await docker(['port', container, '5432/tcp']);
    if (!/^127\.0\.0\.1:\d+$/.test(address)) throw new Error('fixture must listen on loopback');
    const dsn = (role, password, name = database) => `postgres://${role}:${password}@${address}/${name}?sslmode=disable`;
    const env = {
      ...process.env,
      TMPDIR: scratch,
      PSYCHOLOGY_TEST_DB_REQUIRED: '1',
      PSYCHOLOGY_TEST_DB_NAME: database,
      PSYCHOLOGY_TEST_MIGRATION_DB_NAME: migrationDatabase,
      TEST_ADMIN_DATABASE_URL: dsn('phase1_admin', adminPassword),
      TEST_MIGRATION_DATABASE_URL: dsn('phase1_admin', adminPassword, migrationDatabase),
      TEST_APP_DATABASE_URL: dsn('app_user', appPassword),
      TEST_WORKER_DATABASE_URL: dsn('worker_user', workerPassword),
    };
    delete env.DATABASE_URL;
    delete env.WORKER_DATABASE_URL;
    // Serial packages share one disposable database and must not consume each other's outbox items.
    const passed = new Set();
    await run('go', ['test', '-json', '-count=1', '-p', '1', './...'], {
      cwd: api, env, timeout: 300_000, stopGrace: 8000, signal: cancellation.signal,
      onLine(line) {
        const event = JSON.parse(line);
        if (event.Output) process.stdout.write(event.Output);
        if (event.Action === 'pass' && event.Test) passed.add(event.Test);
      },
    });
    const missing = requiredTests.filter(name => !passed.has(name));
    if (missing.length) throw new Error('required live tests did not pass: ' + missing.join(', '));
    console.log(`Verified ${requiredTests.length} mandatory live tests in ${database}`);
  } finally {
    const cleanup = { signal: undefined };
    const owned = await docker(['ps', '-aq', '--filter', `label=${owner}`], cleanup);
    if (owned) await docker(['rm', '--force', '--volumes', ...owned.split('\n')], cleanup);
  }
} finally {
  try {
    if (scratch) await rm(scratch, { recursive: true, force: true });
  } finally {
    cancellation.dispose();
  }
}
