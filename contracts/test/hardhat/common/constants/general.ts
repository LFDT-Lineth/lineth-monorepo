import { ethers } from "hardhat";

export const MAX_UINT32 = BigInt(2 ** 32 - 1);
export const MAX_UINT33 = BigInt(2 ** 33 - 1);
export const HASH_ZERO = ethers.ZeroHash;
export const ADDRESS_ZERO = ethers.ZeroAddress;
export const HASH_WITHOUT_ZERO_FIRST_BYTE = "0xf887bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f";

export const LINEA_ROLLUP_INITIALIZE_SIGNATURE =
  "initialize((bytes32,uint256,uint256,address,uint256,uint256,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[],address,address))";

export const BLS_CURVE_MODULUS = 52435875175126190479447740508185965837690552500527637822603658699938581184513n;

export const BLOCK_COINBASE = "0xc014ba5ec014ba5ec014ba5ec014ba5ec014ba5e";
