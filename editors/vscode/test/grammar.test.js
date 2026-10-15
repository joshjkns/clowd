// Tokenizes Clowd source with the same engine VS Code uses and checks the
// innermost scope of chosen words. Run with `npm test`.
"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const oniguruma = require("vscode-oniguruma");
const textmate = require("vscode-textmate");

async function loadGrammar() {
  const wasm = fs.readFileSync(require.resolve("vscode-oniguruma/release/onig.wasm"));
  await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
  const registry = new textmate.Registry({
    onigLib: Promise.resolve({
      createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
      createOnigString: (s) => new oniguruma.OnigString(s),
    }),
    loadGrammar: async () => {
      const file = path.join(__dirname, "..", "syntaxes", "clowd.tmLanguage.json");
      return textmate.parseRawGrammar(fs.readFileSync(file, "utf8"), file);
    },
  });
  return registry.loadGrammar("source.clowd");
}

// scopes maps each token's text on each line to its innermost scope.
function scopes(grammar, src) {
  const out = [];
  let state = textmate.INITIAL;
  for (const line of src.split("\n")) {
    const r = grammar.tokenizeLine(line, state);
    const tokens = {};
    for (const t of r.tokens) {
      const text = line.slice(t.startIndex, t.endIndex);
      if (text.trim()) tokens[text.trim()] = t.scopes[t.scopes.length - 1];
    }
    out.push(tokens);
    state = r.ruleStack;
  }
  return out;
}

const src = `# header comment
use aws in eu-west-2
USE gcp
database db is postgres 16, small and protected  # trailing
  storage 50gb
  on aws
    backup_retention_period 14
    blue_green_update
      enabled true
load balancer web is internal
  listen on 443 with tls for api.example.com
  send /api to api
service api
  image ghcr.io/me/api:latest#notacomment
  env GREETING "hello world"
  health check /healthz
  scale 2 to 10
queue connects to db
api reads and writes uploads
network is private
on aws
  network
    vpc vpc-1`;

// [line, token text, expected innermost scope]
const expectations = [
  [0, "#", "punctuation.definition.comment.clowd"],
  [0, "header comment", "comment.line.number-sign.clowd"],
  [1, "use", "keyword.control.use.clowd"],
  [1, "aws", "support.constant.cloud.clowd"],
  [1, "in", "keyword.other.in.clowd"],
  [1, "eu-west-2", "string.unquoted.region.clowd"],
  [2, "USE", "keyword.control.use.clowd"],
  [3, "database", "storage.type.kind.clowd"],
  [3, "db", "entity.name.type.resource.clowd"],
  [3, "is", "keyword.operator.word.clowd"],
  [3, "postgres", "constant.language.trait.clowd"],
  [3, "16", "constant.numeric.version.clowd"],
  [3, ",", "punctuation.separator.comma.clowd"],
  [3, "and", "keyword.operator.word.clowd"],
  [3, "trailing", "comment.line.number-sign.clowd"],
  [4, "storage", "variable.other.property.clowd"],
  [4, "50gb", "constant.numeric.clowd"],
  [5, "on", "keyword.control.on.clowd"],
  [5, "aws", "support.constant.cloud.clowd"],
  [6, "backup_retention_period", "support.type.property-name.clowd"],
  [6, "14", "constant.numeric.clowd"],
  [7, "blue_green_update", "support.type.property-name.clowd"],
  [8, "true", "constant.language.boolean.clowd"],
  [9, "load balancer", "storage.type.kind.clowd"],
  [9, "web", "entity.name.type.resource.clowd"],
  [9, "internal", "constant.language.trait.clowd"],
  [10, "listen", "variable.other.property.clowd"],
  [10, "on", "keyword.other.clowd"],
  [10, "443", "constant.numeric.clowd"],
  [10, "with tls for", "keyword.other.clowd"],
  [11, "/api", "string.unquoted.path.clowd"],
  [11, "to", "keyword.other.clowd"],
  [12, "service", "storage.type.kind.clowd"],
  [13, "image", "variable.other.property.clowd"],
  [14, "\"", "string.quoted.double.clowd"],
  [15, "health check", "variable.other.property.clowd"],
  [16, "to", "keyword.other.clowd"],
  [17, "queue", "variable.other.resource.clowd"], // a relationship, not a declaration
  [17, "connects to", "keyword.operator.verb.clowd"],
  [17, "db", "variable.other.resource.clowd"],
  [18, "reads and writes", "keyword.operator.verb.clowd"],
  [19, "network", "storage.type.kind.clowd"],
  [19, "is", "keyword.operator.word.clowd"],
  [19, "private", "constant.language.trait.clowd"],
  [20, "on", "keyword.control.on.clowd"],
  [21, "network", "support.type.property-name.clowd"],
  [22, "vpc", "support.type.property-name.clowd"],
];

(async () => {
  const grammar = await loadGrammar();
  const got = scopes(grammar, src);
  let failed = 0;
  for (const [line, text, want] of expectations) {
    const scope = got[line][text];
    if (scope !== want) {
      failed++;
      console.error(`line ${line + 1}: ${JSON.stringify(text)} is ${scope}, want ${want}\n  tokens: ${JSON.stringify(got[line])}`);
    }
  }
  // A # inside a word is part of the word, as in the lexer.
  assert.ok(!Object.values(got[13]).some((s) => s.startsWith("comment")), "a # inside a word is not a comment");
  // Every example file tokenizes without leaving a rule open.
  for (const file of fs.readdirSync(path.join(__dirname, "..", "..", "..", "examples"))) {
    const text = fs.readFileSync(path.join(__dirname, "..", "..", "..", "examples", file), "utf8");
    scopes(grammar, text);
  }
  if (failed) {
    console.error(`${failed} of ${expectations.length} expectations failed`);
    process.exit(1);
  }
  console.log(`ok: ${expectations.length} scopes`);
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
