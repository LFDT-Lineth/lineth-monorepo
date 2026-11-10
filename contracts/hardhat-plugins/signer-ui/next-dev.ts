/** Lifecycle of the local Next.js signer app (`contracts/signer-ui/`) the bridge launches. */
import { type ChildProcess, execFileSync, spawn } from "node:child_process";
import { readFileSync, unlinkSync } from "node:fs";
import { createServer } from "node:http";
import { Socket } from "node:net";
import { join, resolve } from "node:path";

import { sleep } from "./transactions.js";

export const LOCALHOST = "127.0.0.1";
const SIGNER_UI_DIR = resolve(import.meta.dirname, "../../signer-ui");
const NEXT_DEV_LOCK = join(SIGNER_UI_DIR, ".next", "dev", "lock");
const UI_READY_TIMEOUT_MS = 60 * 1000;
const UI_POLL_INTERVAL_MS = 500;
const UI_PROCESS_KILL_TIMEOUT_MS = 5000;
const EARLY_EXIT_WINDOW_MS = 3500;
const STARTUP_OUTPUT_MAX_CHARS = 8000;

function lsof(args: string[]): string {
  return execFileSync("lsof", args, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"], maxBuffer: 512 * 1024 });
}

/**
 * SIGKILL other processes listening on `port` (Next dev often survives pnpm SIGKILL). Only listeners: a plain `:port`
 * match would also hit clients connected to it (the browser) and this process. Best-effort; no-op on Windows.
 */
export function killTcpListenersOnPort(port: number): void {
  if (process.platform === "win32") {
    return;
  }
  let out: string;
  try {
    out = lsof(["-ti", `tcp:${port}`, "-sTCP:LISTEN"]).trim();
  } catch {
    return; // lsof exits non-zero when nothing listens
  }
  for (const pid of out.split("\n").map((line) => Number.parseInt(line.trim(), 10))) {
    if (Number.isFinite(pid) && pid > 0 && pid !== process.pid) {
      try {
        process.kill(pid, "SIGKILL");
      } catch {
        /* process may have exited */
      }
    }
  }
}

function isPidListeningOnPort(pid: number, port: number): boolean {
  if (process.platform === "win32") {
    return false;
  }
  try {
    return lsof(["-Pan", "-p", String(pid), "-iTCP", "-sTCP:LISTEN", "-Fn"])
      .split("\n")
      .some((line) => line.startsWith("n") && new RegExp(`:${port}(?:$|->)`).test(line.slice(1)));
  } catch {
    return false;
  }
}

/**
 * Next.js 16+ keeps a singleton dev server via `.next/dev/lock` (JSON with pid/port). A stale lock or orphan PID makes
 * `next dev` exit immediately. The lock's PID is signalled only if it still listens on the lock's port (PIDs and ports
 * can be reused by unrelated processes).
 */
export function removeNextDevLock({ stopOwner }: { stopOwner: boolean }): void {
  // No existence check: both the read and the unlink tolerate a missing lock.
  if (stopOwner) {
    try {
      const { pid, port } = JSON.parse(readFileSync(NEXT_DEV_LOCK, "utf8")) as { pid?: number; port?: number };
      if (
        typeof pid === "number" &&
        pid > 0 &&
        typeof port === "number" &&
        port > 0 &&
        isPidListeningOnPort(pid, port)
      ) {
        process.kill(pid, "SIGTERM");
      }
    } catch {
      /* invalid lock, or the process cannot be signalled */
    }
  }
  try {
    unlinkSync(NEXT_DEV_LOCK);
  } catch {
    /* lock recreated or permission */
  }
}

export async function getFreePort(): Promise<number> {
  return await new Promise((resolvePort, reject) => {
    const server = createServer();
    server.on("error", reject);
    server.listen(0, LOCALHOST, () => {
      const address = server.address();
      const port = typeof address === "object" && address ? address.port : undefined;
      server.close((error) => {
        if (error) {
          reject(error);
        } else if (!port) {
          reject(new Error("Failed to allocate a local port for HARDHAT_SIGNER_UI"));
        } else {
          resolvePort(port);
        }
      });
    });
  });
}

/**
 * Starts `next dev` for the signer app on `uiPort` and fails fast (with its output) if it exits right away. The child
 * and its pipes are unref'd so a finished Hardhat command can exit.
 */
export async function startNextDev(
  uiPort: number,
  onUnexpectedExit: (code: number | null) => void,
): Promise<ChildProcess> {
  removeNextDevLock({ stopOwner: true });
  await sleep(400);

  // Never use "pipe" without draining: Next/Turbopack logs fill buffers and the child blocks before listening.
  const captureOutput = process.env.HARDHAT_SIGNER_UI_DEBUG !== "true";
  let output = "";
  const child = spawn(
    "pnpm",
    ["--dir", SIGNER_UI_DIR, "exec", "next", "dev", "--hostname", LOCALHOST, "--port", String(uiPort)],
    { cwd: SIGNER_UI_DIR, env: { ...process.env }, stdio: captureOutput ? "pipe" : "inherit" },
  );
  const append = (chunk: string | Buffer) => {
    output = `${output}${chunk.toString()}`.slice(-STARTUP_OUTPUT_MAX_CHARS);
  };
  child.stdout?.on("data", append);
  child.stderr?.on("data", append);
  child.on("exit", (code) => {
    if (code !== 0) {
      onUnexpectedExit(code);
    }
  });

  const earlyExitCode = await new Promise<number | null>((resolveEarly) => {
    const timer = setTimeout(() => {
      child.off("exit", onExit);
      resolveEarly(null);
    }, EARLY_EXIT_WINDOW_MS);
    const onExit = (code: number | null) => {
      clearTimeout(timer);
      resolveEarly(code);
    };
    child.once("exit", onExit);
  });
  if (earlyExitCode !== null) {
    const trimmed = output.trim();
    throw new Error(
      `Signer UI (next dev) exited early with code ${earlyExitCode}. ` +
        "This can be caused by a stale .next/dev/lock, another running `next dev` for contracts/signer-ui, or missing signer-ui dependencies after a clean/install change." +
        (trimmed.length > 0 ? ` Early output:\n${trimmed}` : " No startup output was captured."),
    );
  }

  child.unref();
  for (const stream of [child.stdin, child.stdout, child.stderr]) {
    if (stream instanceof Socket) {
      stream.unref();
    }
  }
  return child;
}

/** SIGTERM, then SIGKILL after a timeout; also kills leftover listeners on `uiPort` and removes the lock. */
export async function stopNextDev(child: ChildProcess | undefined, uiPort: number | undefined): Promise<void> {
  if (child) {
    await new Promise<void>((resolveStop) => {
      const timer = setTimeout(() => {
        try {
          child.kill("SIGKILL");
        } catch {
          /* process may already be gone */
        }
        resolveStop();
      }, UI_PROCESS_KILL_TIMEOUT_MS);
      child.once("exit", () => {
        clearTimeout(timer);
        resolveStop();
      });
      try {
        child.kill("SIGTERM");
      } catch {
        clearTimeout(timer);
        resolveStop();
      }
    });
  }
  if (uiPort !== undefined) {
    killTcpListenersOnPort(uiPort);
  }
  removeNextDevLock({ stopOwner: false });
}

export async function waitForHttpOk(url: string): Promise<void> {
  const deadline = Date.now() + UI_READY_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(url)).ok) {
        return;
      }
    } catch {
      // Keep polling until the timeout expires.
    }
    await sleep(UI_POLL_INTERVAL_MS);
  }
  throw new Error(`Timed out while waiting for the signer UI to start at ${url}`);
}

export function openBrowser(url: string): void {
  if (process.env.HARDHAT_SIGNER_UI_OPEN_BROWSER === "false") {
    return;
  }
  const child = spawn(process.platform === "darwin" ? "open" : "xdg-open", [url], { stdio: "ignore", detached: true });
  child.on("error", (error: Error) => {
    console.warn(
      `HARDHAT_SIGNER_UI: failed to open browser automatically (${error.message}). Open this URL manually: ${url}`,
    );
  });
  child.unref();
}
