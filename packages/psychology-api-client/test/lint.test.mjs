import test from 'node:test';
import assert from 'node:assert';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

test('PSY-5.19: pnpm lint succeeds on valid codebase and fails with non-zero exit code on intentional violation', () => {
  // 1. Verify that running lint on valid codebase succeeds with exit code 0
  const validResult = spawnSync('node', ['../../scripts/lint.mjs'], {
    cwd: path.resolve('.'),
    encoding: 'utf8',
  });
  assert.strictEqual(validResult.status, 0, `Valid codebase lint must exit with 0: ${validResult.stderr}`);
  assert.match(validResult.stdout, /✔ Lint passed/, 'Must log success message');

  // 2. Introduce an intentional ts-ignore and debugger violation
  const testViolationFile = path.resolve('./src/__temp_violation_test__.ts');
  const tsIgnoreDirective = '// @ts' + '-ignore';
  const debuggerKeyword = 'debug' + 'ger;';
  try {
    fs.writeFileSync(
      testViolationFile,
      `// Intentional lint violation test\n${tsIgnoreDirective}\nexport const x = 1;\n${debuggerKeyword}\n`,
      'utf8'
    );

    // 3. Verify that running lint fails with non-zero exit code
    const invalidResult = spawnSync('node', ['../../scripts/lint.mjs'], {
      cwd: path.resolve('.'),
      encoding: 'utf8',
    });

    assert.notStrictEqual(invalidResult.status, 0, 'Lint must fail with non-zero exit code on intentional violation');
    assert.match(invalidResult.stderr, /NO_TS_IGNORE/, 'Must report NO_TS_IGNORE violation');
    assert.match(invalidResult.stderr, /NO_DEBUGGER/, 'Must report NO_DEBUGGER violation');
  } finally {
    if (fs.existsSync(testViolationFile)) {
      fs.unlinkSync(testViolationFile);
    }
  }

  // 4. Test empty catch block violation (Rule 14 in AGENTS.md)
  const emptyCatchBlock = 'cat' + 'ch () {}';
  try {
    fs.writeFileSync(
      testViolationFile,
      `export function testCatch() {\n  try {\n    return 1;\n  } ${emptyCatchBlock}\n}\n`,
      'utf8'
    );

    const emptyCatchResult = spawnSync('node', ['../../scripts/lint.mjs'], {
      cwd: path.resolve('.'),
      encoding: 'utf8',
    });

    assert.notStrictEqual(emptyCatchResult.status, 0, 'Lint must fail on empty catch block');
    assert.match(emptyCatchResult.stderr, /NO_EMPTY_CATCH/, 'Must report NO_EMPTY_CATCH violation');
  } finally {
    if (fs.existsSync(testViolationFile)) {
      fs.unlinkSync(testViolationFile);
    }
  }

  // 5. Test merge conflict marker violation
  const conflictStart = '<'.repeat(7) + ' HEAD';
  const conflictSep = '='.repeat(7);
  const conflictEnd = '>'.repeat(7) + ' branch';
  try {
    fs.writeFileSync(
      testViolationFile,
      `${conflictStart}\nexport const a = 1;\n${conflictSep}\nexport const a = 2;\n${conflictEnd}\n`,
      'utf8'
    );

    const conflictResult = spawnSync('node', ['../../scripts/lint.mjs'], {
      cwd: path.resolve('.'),
      encoding: 'utf8',
    });

    assert.notStrictEqual(conflictResult.status, 0, 'Lint must fail on merge conflict markers');
    assert.match(conflictResult.stderr, /NO_CONFLICT_MARKERS/, 'Must report NO_CONFLICT_MARKERS violation');
  } finally {
    if (fs.existsSync(testViolationFile)) {
      fs.unlinkSync(testViolationFile);
    }
  }
});
