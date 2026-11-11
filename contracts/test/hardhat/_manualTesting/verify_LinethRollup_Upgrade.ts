import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre from "hardhat";
import { describe, it } from "node:test";

// pnpm exec hardhat test nodejs --network mainnet <this file>
// THIS IS A MANUAL TEST TO VERIFY MAINNET DOES NOT BREAK
// ALSO SEE PR FOR LAYOUT TABLE
describe.skip("LinethRollup Upgrade from ZkEvmMainnet", () => {
  const mainnetProxyAddress = "0xd19d4B5d358258f05D7B411E21A1460D11B0876F";

  describe("Collision", () => {
    it("Does not collide", async () => {
      const connection = await hre.network.getOrCreate();
      const oz = await upgrades(hre, connection);

      console.log("Importing contract");
      await oz.forceImport(mainnetProxyAddress, "ZkEvmV2Mainnet", {
        kind: "transparent",
      });

      await oz.validateUpgrade(mainnetProxyAddress, "LinethRollup");
    });
  });
});
