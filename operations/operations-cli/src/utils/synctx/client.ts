import { ethers } from "ethers";

export type ClientApi = {
  [key: string]: {
    api: string;
    params: Array<unknown>;
  };
};

export const getClientType = async (nodeProvider: ethers.JsonRpcProvider): Promise<string> => {
  const res: string = await nodeProvider.send("web3_clientVersion", []);
  const clientType = res.slice(0, 4).toLowerCase();
  if (!["geth", "besu"].includes(clientType)) {
    throw new Error(`Invalid node client type, must be either geth or besu`);
  }
  return clientType;
};
