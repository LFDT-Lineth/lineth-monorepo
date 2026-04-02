import { parseVerifyTaskArgs } from "./verify-task-args";

async function main() {
  const task = process.env.HARDHAT_VERIFY_TASK;
  const rawArgs = process.env.HARDHAT_VERIFY_ARGS;

  if (!task) {
    throw new Error("Missing HARDHAT_VERIFY_TASK.");
  }
  if (!rawArgs) {
    throw new Error("Missing HARDHAT_VERIFY_ARGS.");
  }

  const args = parseVerifyTaskArgs(rawArgs);
  const hreModule = await import("hardhat");
  const run =
    hreModule.run ??
    (
      hreModule.default as
        | { run?: ((taskName: string, taskArgs?: Record<string, unknown>) => Promise<unknown>) | undefined }
        | undefined
    )?.run;

  if (!run) {
    throw new Error("Hardhat runtime not available in verification child process.");
  }

  await run(task, args);
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
