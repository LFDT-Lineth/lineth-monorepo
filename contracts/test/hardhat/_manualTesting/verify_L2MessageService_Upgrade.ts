import { ethers, upgrades } from "hardhat";

// npx hardhat test --network linea_mainnet
// THIS IS A MANUAL TEST TO VERIFY LINEA MAINNET DOES NOT BREAK
// ALSO SEE PR FOR LAYOUT TABLE
describe.skip("L2MessageService Upgrade from L2MessageService Linea Mainnet", () => {
  const mainnetProxyAddress = "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec";

  describe("Collision", () => {
    it("Does not collide", async () => {
      const contract = await ethers.getContractFactory("L2MessageServiceLineaMainnet");

      console.log("Importing contract");
      await upgrades.forceImport(mainnetProxyAddress, contract, {
        kind: "transparent",
      });

      const l2MessageServiceFactory = await ethers.getContractFactory("L2MessageService");
      await upgrades.validateUpgrade(mainnetProxyAddress, l2MessageServiceFactory);
    });
  });
});
