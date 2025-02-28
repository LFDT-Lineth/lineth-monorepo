import { HDNodeWallet, Wallet } from "ethers";
import { config, ethers } from "hardhat";
import { HardhatNetworkHDAccountsConfig } from "hardhat/types";

export const getWalletForIndex = (index: number) => {
  const accounts = config.networks.hardhat.accounts as HardhatNetworkHDAccountsConfig;
  const signer = HDNodeWallet.fromPhrase(accounts.mnemonic, "", `m/44'/60'/0'/0/${index}`);
  return new Wallet(signer.privateKey, ethers.provider);
};
