import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre from "hardhat";
import { describe, it } from "node:test";

// pnpm exec hardhat test nodejs --network linea_mainnet <this file>
// THIS IS A MANUAL TEST TO VERIFY LINEA MAINNET DOES NOT BREAK
// ALSO SEE PR FOR LAYOUT TABLE
describe.skip("L2MessageService Upgrade from L2MessageService Linea Mainnet", () => {
  const mainnetProxyAddress = "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec";

  describe("Collision", () => {
    it("Does not collide", async () => {
      const connection = await hre.network.getOrCreate();
      const oz = await upgrades(hre, connection);

      console.log("Importing contract");
      await oz.forceImport(mainnetProxyAddress, "L2MessageServiceLineaMainnet", {
        kind: "transparent",
      });

      await oz.validateUpgrade(mainnetProxyAddress, "L2MessageService");
    });
  });
});
