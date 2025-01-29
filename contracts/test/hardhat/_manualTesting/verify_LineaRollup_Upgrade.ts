import { ethers, upgrades } from "hardhat";

// npx hardhat test --network mainnet
// THIS IS A MANUAL TEST TO VERIFY MAINNET DOES NOT BREAK
// ALSO SEE PR FOR LAYOUT TABLE
describe.skip("LineaRollup Upgrade from ZkEvmMainnet", () => {
  const mainnetProxyAddress = "0xd19d4B5d358258f05D7B411E21A1460D11B0876F";

  describe("Collision", () => {
    it("Does not collide", async () => {
      const contract = await ethers.getContractFactory("ZkEvmV2Mainnet");

      console.log("Importing contract");
      await upgrades.forceImport(mainnetProxyAddress, contract, {
        kind: "transparent",
      });

      const LineaRollupFactory = await ethers.getContractFactory("LineaRollup");
      await upgrades.validateUpgrade(mainnetProxyAddress, LineaRollupFactory);
    });
  });
});
