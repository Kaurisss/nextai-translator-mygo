#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

// Decide the next release version from git commits since the last v* tag.
export function nextVersion(currentVersion, lastTag, commits) {
  if (!lastTag) return currentVersion;
  const match = lastTag.match(/^v?(\d+)\.(\d+)\.(\d+)$/);
  if (!match) {
    throw new Error(`Invalid tag version: ${lastTag}`);
  }
  let [, major, minor, patch] = match.map(Number);

  let level = null;
  for (const commit of commits) {
    const lines = commit.trim().split(/\r?\n/);
    const subject = (lines[0] || "").trim();
    const body = lines.slice(1).join("\n");
    if (!subject || subject.startsWith("chore(release):") || commit.includes("[skip release]")) {
      continue;
    }
    const isBreaking = /^[a-z]+(\([^)]+\))?!:/i.test(subject) || /^BREAKING[ -]CHANGE:/m.test(body);
    if (isBreaking) {
      level = "breaking";
      break;
    }
    if (/^(docs|chore|ci|build|style|test)(\([^)]+\))?:/i.test(subject)) {
      continue;
    }
    if (/^feat(\([^)]+\))?:/i.test(subject)) {
      level = level || "feat";
    } else {
      level = level || "patch";
    }
  }

  if (!level) return null;
  if (level === "breaking") {
    return major === 0 ? `${major}.${minor + 1}.0` : `${major + 1}.0.0`;
  }
  if (level === "feat") {
    return `${major}.${minor + 1}.0`;
  }
  return `${major}.${minor}.${patch + 1}`;
}

export function bumpFiles(version, mygoPath = "mygo.json", issPath = "installer/windows.iss") {
  let changed = false;
  const mygoRaw = readFileSync(mygoPath, "utf8");
  const mygo = JSON.parse(mygoRaw);
  if (mygo.version !== version) {
    const updated = mygoRaw.replace(/"version"\s*:\s*"[^"]+"/, `"version": "${version}"`);
    writeFileSync(mygoPath, updated, "utf8");
    changed = true;
  }
  const issRaw = readFileSync(issPath, "utf8");
  const issUpdated = issRaw.replace(/(#define AppVersion ")[^"]+(")/, `$1${version}$2`);
  if (issUpdated !== issRaw) {
    writeFileSync(issPath, issUpdated, "utf8");
    changed = true;
  }
  return changed;
}

function git(...args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const mygo = JSON.parse(readFileSync("mygo.json", "utf8"));
  const lastTag = git("tag", "--merged", "HEAD", "--list", "v*", "--sort=-v:refname")
    .split(/\r?\n/)
    .find(Boolean) || "";
  const rawLog = lastTag
    ? git("log", `${lastTag}..HEAD`, "--format=%B%x1e")
    : "";
  const commits = rawLog ? rawLog.split("\x1e").map((s) => s.trim()).filter(Boolean) : [];
  const version = nextVersion(mygo.version, lastTag, commits);
  const apply = process.argv.includes("--apply");
  const changed = version && apply ? bumpFiles(version) : false;
  process.stdout.write(JSON.stringify({ version, tag: version ? `v${version}` : null, lastTag, changed }) + "\n");
}
