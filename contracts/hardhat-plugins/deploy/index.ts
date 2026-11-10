import { overrideTask } from "hardhat/config";
import { definePlugin } from "hardhat/plugins";

/**
 * Wraps `hardhat ignition deploy` with module parameters from `<Module>.deploy.ts`, OpenZeppelin Upgrades checks,
 * browser signing and post-deploy steps (see `./deploy-action.ts`). Accepts a module name instead of a path:
 * `hardhat ignition deploy LinethRollup --network sepolia`.
 */
export default definePlugin({
  id: "lineth-deploy",
  dependencies: () => [
    import("../signer-ui/index.js"),
    import("@nomicfoundation/hardhat-viem"),
    import("@nomicfoundation/hardhat-ignition"),
    import("@openzeppelin/hardhat-upgrades"),
  ],
  tasks: [
    overrideTask(["ignition", "deploy"])
      .setAction(() => import("./deploy-action.js"))
      .build(),
  ],
});
