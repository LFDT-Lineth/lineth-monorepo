import { ZeroAddress, ZeroHash } from "ethers";

export const HASH_ZERO = ZeroHash;
export const ADDRESS_ZERO = ZeroAddress;

export const LINEA_ROLLUP_INITIALIZE_SIGNATURE =
  "initialize((bytes32,uint256,uint256,address,uint256,uint256,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[],address,address),address,address)";

export const VALIDIUM_INITIALIZE_SIGNATURE =
  "initialize((bytes32,uint256,uint256,address,uint256,uint256,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[],address,address))";

export const L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE =
  "initialize(uint256,uint256,address,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[])";

export const EMPTY_INITIALIZE_SIGNATURE = "initialize()";

export const ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE =
  "initializeRolesAndStorageVariables(uint256,address,address,address,address,address,address,address,address,address)";

export const YIELD_MANAGER_INITIALIZE_SIGNATURE =
  "initialize(((uint8,bytes32)[],(uint8,bytes32)[],(address,bytes32)[],address[],address,uint16,uint16,uint256,uint256))";

export const DEAD_ADDRESS = "0x000000000000000000000000000000000000dEaD";
