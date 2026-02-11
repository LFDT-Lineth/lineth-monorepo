import { etherToWei } from "@consensys/linea-shared-utils";

import { sendTransactionsToGenerateTrafficWithInterval } from "../../common/utils";

import type { TestContext } from "../setup";

type StartTrafficOptions = {
  pollingIntervalMs?: number;
};

export async function startL2TrafficGeneration(
  context: TestContext,
  options: StartTrafficOptions = {},
): Promise<() => void> {
  const pollingIntervalMs = options.pollingIntervalMs ?? 2_000;
  const pollingAccount = await context.getL2AccountManager().generateAccount(etherToWei("200"));
  const walletClient = context.l2WalletClient({ account: pollingAccount });
  const publicClient = context.l2PublicClient();

  return sendTransactionsToGenerateTrafficWithInterval(walletClient, publicClient, {
    pollingInterval: pollingIntervalMs,
  });
}
