import { ethers } from "hardhat";

export const MAX_UINT32 = BigInt(2 ** 32 - 1);
export const MAX_UINT33 = BigInt(2 ** 33 - 1);
export const HASH_ZERO = ethers.ZeroHash;
export const ADDRESS_ZERO = ethers.ZeroAddress;
export const HASH_WITHOUT_ZERO_FIRST_BYTE = "0xf887bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f";

export const VALIDIUM_INITIALIZE_SIGNATURE =
  "initialize((bytes32,uint256,uint256,address,uint256,uint256,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[],address,address,address))";

export const LINEA_ROLLUP_INITIALIZE_SIGNATURE =
  "initialize((bytes32,uint256,uint256,address,uint256,uint256,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[],address,address,address),address,address)";

export const BLOCK_COINBASE = "0xc014ba5ec014ba5ec014ba5ec014ba5ec014ba5e";

export const ONE_GWEI = ethers.parseUnits("1", "gwei");
export const ONE_ETHER = ethers.parseEther("1");
export const ONE_THOUSAND_ETHER = ethers.parseEther("1000");
export const UINT64_MAX = BigInt("18446744073709551615");

// Token Bridge status constants
export const TOKEN_BRIDGE_RESERVED_STATUS = ethers.getAddress("0x0000000000000000000000000000000000000111");
export const TOKEN_BRIDGE_NATIVE_STATUS = ethers.getAddress("0x0000000000000000000000000000000000000222");
export const TOKEN_BRIDGE_DEPLOYED_STATUS = ethers.getAddress("0x0000000000000000000000000000000000000333");
