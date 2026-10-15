"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { resolve } = require("../server-path");

// makeBin creates an executable named name in a new folder under root.
function makeBin(root, dir, name = "clowd") {
  const d = path.join(root, dir);
  fs.mkdirSync(d, { recursive: true });
  const file = path.join(d, name);
  fs.writeFileSync(file, "#!/bin/sh\n", { mode: 0o755 });
  return file;
}

test("resolve", { skip: process.platform === "win32" }, (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "clowd-path-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const home = path.join(root, "home");
  const onPath = makeBin(root, "usr/bin");
  const inGoBin = makeBin(home, "go/bin");
  const inGOBIN = makeBin(root, "gobin");
  const named = makeBin(root, "usr/bin", "clowd-dev");
  fs.writeFileSync(path.join(root, "notexec"), "", { mode: 0o644 });

  const cases = [
    ["on PATH first", "clowd", { PATH: path.join(root, "usr/bin") }, onPath],
    ["~/go/bin when not on PATH", "clowd", { PATH: "/nowhere" }, inGoBin],
    ["empty setting means clowd", "", { PATH: "/nowhere" }, inGoBin],
    ["$GOBIN wins over ~/go/bin", "clowd", { PATH: "/nowhere", GOBIN: path.join(root, "gobin") }, inGOBIN],
    ["$GOPATH/bin", "clowd", { PATH: "", GOPATH: home + path.delimiter + "/x" }, path.join(home, "bin", "clowd")],
    ["another bare name", "clowd-dev", { PATH: path.join(root, "usr/bin") }, named],
    ["a path", inGOBIN, { PATH: "" }, inGOBIN],
    ["a ~ path", "~/go/bin/clowd", { PATH: "" }, inGoBin],
    ["a missing path", path.join(root, "nope"), { PATH: "" }, undefined],
    ["not executable", path.join(root, "notexec"), { PATH: "" }, undefined],
    ["nowhere", "clowd", { PATH: "/nowhere", GOBIN: "/nowhere/either" }, undefined],
  ];
  // $GOPATH/bin needs its own binary.
  makeBin(home, "bin");
  for (const [name, setting, env, want] of cases) {
    const got = resolve(setting, env, home, "darwin");
    assert.equal(got.command, want, name);
    assert.ok(got.searched.length > 0, `${name}: searched nothing`);
  }
});
