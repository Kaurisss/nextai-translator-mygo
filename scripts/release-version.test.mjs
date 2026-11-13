import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { bumpFiles, nextVersion } from "./release-version.mjs";

test("initial release uses current mygo.json version when no tag exists", () => {
  assert.equal(nextVersion("0.0.1", "", ["feat: init"]), "0.0.1");
});

test("skips non-releasable commits and release bot commits", () => {
  assert.equal(
    nextVersion("0.0.1", "v0.0.1", ["docs: update readme", "ci: tweak", "chore(release): v0.0.1 [skip ci]"]),
    null,
  );
});

test("bumps patch for fix and plain commits", () => {
  assert.equal(nextVersion("0.0.1", "v0.0.1", ["fix: ocr crash"]), "0.0.2");
  assert.equal(nextVersion("0.0.1", "v0.0.1", ["修复截图报错"]), "0.0.2");
});

test("bumps minor for feat and pre-1.0 breaking, major for >=1.0 breaking", () => {
  assert.equal(nextVersion("0.0.1", "v0.0.1", ["feat: add workflow"]), "0.1.0");
  assert.equal(nextVersion("0.1.0", "v0.1.0", ["feat!: drop legacy config"]), "0.2.0");
  assert.equal(nextVersion("1.2.3", "v1.2.3", ["fix: bug\n\nBREAKING CHANGE: api"]), "2.0.0");
});

test("bumpFiles updates mygo.json and installer/windows.iss idempotently", () => {
  const dir = mkdtempSync(join(tmpdir(), "release-ver-"));
  try {
    const mygo = join(dir, "mygo.json");
    const iss = join(dir, "windows.iss");
    writeFileSync(mygo, '{\n  "version": "0.0.1"\n}\n');
    writeFileSync(iss, '#ifndef AppVersion\n  #define AppVersion "0.0.1"\n#endif\n');
    assert.equal(bumpFiles("0.0.1", mygo, iss), false);
    assert.equal(bumpFiles("0.0.2", mygo, iss), true);
    assert.match(readFileSync(mygo, "utf8"), /"version": "0\.0\.2"/);
    assert.match(readFileSync(iss, "utf8"), /#define AppVersion "0\.0\.2"/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
