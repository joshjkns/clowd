// The Clowd extension: the grammar does the highlighting, and everything
// else (diagnostics, hovers, completion, formatting) comes from the
// language server built into the clowd binary, `clowd lsp`.
"use strict";

const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");
const { resolve } = require("./server-path");

/** @type {LanguageClient | undefined} */
let client;

/** @param {vscode.ExtensionContext} context */
async function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand("clowd.restartServer", restart),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("clowd.path")) {
        restart();
      }
    }),
  );
  await start();
}

async function start() {
  const setting = vscode.workspace.getConfiguration("clowd").get("path") || "clowd";
  // Find the binary first, so a missing one gets one clear message
  // rather than the client library's "spawn clowd ENOENT".
  const { command, searched } = resolve(setting);
  if (!command) {
    notFound(`couldn't find \`${setting}\`. Looked in: ${searched.join(", ")}.`);
    return;
  }
  client = new LanguageClient(
    "clowd",
    "Clowd",
    // TransportKind.stdio adds --stdio, which `clowd lsp` accepts.
    { command, args: ["lsp"], transport: TransportKind.stdio },
    {
      documentSelector: [
        { scheme: "file", language: "clowd" },
        { scheme: "untitled", language: "clowd" },
      ],
    },
  );
  try {
    await client.start();
  } catch (err) {
    client = undefined;
    notFound(`couldn't start \`${command} lsp\`: ${err instanceof Error ? err.message : err}.`);
  }
}

// notFound explains why the server isn't running and how to fix it.
async function notFound(problem) {
  const choice = await vscode.window.showErrorMessage(
    `Clowd: ${problem} Install it with \`go install github.com/joshjkns/clowd/cmd/clowd@latest\`, ` +
      "or set `clowd.path` to the binary.",
    "Open Settings",
  );
  if (choice === "Open Settings") {
    vscode.commands.executeCommand("workbench.action.openSettings", "clowd.path");
  }
}

async function restart() {
  await stop();
  await start();
}

async function stop() {
  if (client) {
    const c = client;
    client = undefined;
    await c.stop();
  }
}

function deactivate() {
  return stop();
}

module.exports = { activate, deactivate };
