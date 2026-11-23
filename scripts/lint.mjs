#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { execSync } from 'node:child_process';

const targetDir = process.argv[2] ? path.resolve(process.argv[2]) : process.cwd();

// Scan files recursively
function getFiles(dir, fileList = []) {
  if (!fs.existsSync(dir)) return fileList;
  const entries = fs.readdirSync(dir, { withFileTypes: true });

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (
        entry.name === 'node_modules' ||
        entry.name === 'dist' ||
        entry.name === '.astro' ||
        entry.name === '.git'
      ) {
        continue;
      }
      getFiles(fullPath, fileList);
    } else if (entry.isFile()) {
      const ext = path.extname(entry.name);
      if (['.ts', '.tsx', '.js', '.mjs', '.astro', '.json', '.css'].includes(ext)) {
        fileList.push(fullPath);
      }
    }
  }
  return fileList;
}

const files = getFiles(targetDir);
let hasErrors = false;
const errors = [];

function addError(file, lineNum, rule, message) {
  hasErrors = true;
  const relPath = path.relative(process.cwd(), file);
  errors.push(`  [${rule}] ${relPath}:${lineNum} - ${message}`);
}

for (const file of files) {
  const content = fs.readFileSync(file, 'utf8');
  const lines = content.split('\n');

  lines.forEach((line, idx) => {
    const lineNum = idx + 1;

    // Rule 1: No merge conflict markers
    if (/^(<{7}|={7}|>{7})(\s|$)/.test(line)) {
      addError(file, lineNum, 'NO_CONFLICT_MARKERS', 'Merge conflict marker found');
    }

    // Rule 2: No @ts-ignore or @ts-nocheck directives
    if (/^\s*\/\/\s*@ts-(?:ignore|nocheck)\b/.test(line)) {
      addError(file, lineNum, 'NO_TS_IGNORE', 'Forbidden @ts-ignore or @ts-nocheck directive');
    }

    // Rule 3: No debugger statements
    if (/^\s*debugger\s*;?$/.test(line.trim())) {
      addError(file, lineNum, 'NO_DEBUGGER', 'Leftover debugger statement found');
    }

    // Rule 4: No hardcoded secrets / private keys
    if (/BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY/.test(line)) {
      addError(file, lineNum, 'NO_HARDCODED_SECRETS', 'Hardcoded private key detected');
    }
  });

  // Rule 5: No completely empty catch blocks: catch (...) {} or catch {} with only whitespace
  const emptyCatchRegex = /catch\s*(?:\([^)]*\))?\s*\{\s*\}/g;
  let match;
  while ((match = emptyCatchRegex.exec(content)) !== null) {
    const preContent = content.slice(0, match.index);
    const lineNum = preContent.split('\n').length;
    addError(file, lineNum, 'NO_EMPTY_CATCH', 'Empty catch block is forbidden (AGENTS.md Rule 14)');
  }
}

if (hasErrors) {
  console.error(`\n❌ Lint failed with ${errors.length} error(s):`);
  errors.forEach((e) => console.error(e));
  process.exit(1);
}

// Run typecheck / Astro check if configs exist
const hasTsConfig = fs.existsSync(path.join(targetDir, 'tsconfig.json'));
const hasAstroConfig = fs.existsSync(path.join(targetDir, 'astro.config.mjs'));

try {
  if (hasAstroConfig) {
    execSync('npx astro check', { cwd: targetDir, stdio: 'inherit' });
  }
  if (hasTsConfig) {
    execSync('npx tsc --noEmit', { cwd: targetDir, stdio: 'inherit' });
  }
} catch {
  process.exit(1);
}

console.log(`✔ Lint passed: ${files.length} files checked in ${path.basename(targetDir)}`);
process.exit(0);
