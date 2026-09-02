// Thin wrapper around vitest-websocket-mock so route tests don't depend
// on the upstream API directly. The lifecycle (clean / close) belongs in
// each test's afterEach to keep it explicit.

import WS from "vitest-websocket-mock";
import { screenshotLogLines } from "./screenshotData";

export function mockWS(url: string) {
  const server = new WS(url);
  return {
    server,
    sendLine(line: string) {
      server.send(line);
    },
    // Helper to stream screenshot log lines for console/logs tabs
    streamScreenshotLogs() {
      screenshotLogLines.forEach((line) => {
        server.send(line);
      });
    },
    close() {
      WS.clean();
    },
  };
}

export function cleanupWS() {
  WS.clean();
}
