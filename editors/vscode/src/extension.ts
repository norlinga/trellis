import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export function activate(_context: vscode.ExtensionContext): void {
  const config = vscode.workspace.getConfiguration("trellis");
  const executable = config.get<string>("executable", "trellis");

  const serverOptions: ServerOptions = {
    run: { command: executable, args: ["lsp"], transport: TransportKind.stdio },
    debug: { command: executable, args: ["lsp"], transport: TransportKind.stdio },
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "trellis" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.trellis"),
    },
  };

  client = new LanguageClient(
    "trellis",
    "Trellis Language Server",
    serverOptions,
    clientOptions
  );

  client.start();
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
