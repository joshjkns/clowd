// Finds the clowd binary. It has no dependency on vscode, so it can be
// tested with plain node.
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

function isExecutable(file) {
  try {
    fs.accessSync(file, fs.constants.X_OK);
    return fs.statSync(file).isFile();
  } catch {
    return false;
  }
}

// goBinDirs are where `go install` puts binaries: $GOBIN, else the bin
// folder of each $GOPATH entry, else ~/go/bin.
function goBinDirs(env, home) {
  if (env.GOBIN) return [env.GOBIN];
  if (env.GOPATH) return env.GOPATH.split(path.delimiter).filter(Boolean).map((p) => path.join(p, "bin"));
  return [path.join(home, "go", "bin")];
}

/**
 * resolve returns the absolute path of the binary `setting` names, or
 * undefined with the places it looked. A setting with a slash is a path;
 * a bare name is looked up on PATH and then where `go install` puts
 * binaries, because `go install`'s folder is often not on PATH, and an
 * editor started from the Dock doesn't see the shell's PATH anyway.
 *
 * @returns {{ command?: string, searched: string[] }}
 */
function resolve(setting, env = process.env, home = os.homedir(), platform = process.platform) {
  const name = setting || "clowd";
  if (name.includes("/") || name.includes("\\")) {
    const file = name.startsWith("~/") ? path.join(home, name.slice(2)) : name;
    return isExecutable(file) ? { command: file, searched: [file] } : { searched: [file] };
  }
  const exe = platform === "win32" && !name.endsWith(".exe") ? name + ".exe" : name;
  const dirs = [...(env.PATH || "").split(path.delimiter).filter(Boolean), ...goBinDirs(env, home)];
  const searched = [];
  for (const dir of dirs) {
    const file = path.join(dir, exe);
    if (searched.includes(file)) continue;
    searched.push(file);
    if (isExecutable(file)) return { command: file, searched };
  }
  return { searched };
}

module.exports = { resolve };
