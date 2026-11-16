import { globalFlag, task } from "hardhat/config";
import { definePlugin } from "hardhat/plugins";
import { ArgumentType } from "hardhat/types/arguments";

export type * from "./type-extensions.js";

/**
 * Browser-wallet signing for Hardhat tasks and scripts. `--signer-ui` (or `HARDHAT_SIGNER_UI=true`) enables it; tasks
 * opt in by wrapping their work in `withSignerUi(hre, label, fn)` (`./session.ts`). See `contracts/signer-ui/README.md`.
 */
export default definePlugin({
  id: "lineth-signer-ui",
  globalOptions: [
    globalFlag({
      name: "signerUi",
      description: "Sign transactions in the browser wallet (signer UI) instead of DEPLOYER_PRIVATE_KEY",
    }),
  ],
  tasks: [
    task("deploy:recover", "Resolves an Ignition deployment stuck on an unsent transaction (wallet rejection, Ctrl-C)")
      .addOption({ name: "deploymentId", type: ArgumentType.STRING_WITHOUT_DEFAULT, defaultValue: undefined })
      .setAction(() => import("./recover.js"))
      .build(),
  ],
});
