// The Clowd extension: the grammar does the highlighting, and everything
// else (diagnostics, hovers, completion, formatting) comes from the
// language server built into the clowd binary, `clowd lsp`.
"use strict";

const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

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
  const command = vscode.workspace.getConfiguration("clowd").get("path") || "clowd";
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
    const choice = await vscode.window.showErrorMessage(
      `Clowd: couldn't start the language server with \`${command} lsp\`. ` +
        "Install clowd, or set `clowd.path` to the binary.",
      "Open Settings",
    );
    if (choice === "Open Settings") {
      vscode.commands.executeCommand("workbench.action.openSettings", "clowd.path");
    }
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
