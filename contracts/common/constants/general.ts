import { ZeroHash } from "ethers";

export const HASH_ZERO = ZeroHash;

export const LINEA_ROLLUP_INITIALIZE_SIGNATURE =
  "initialize((bytes32,uint256,uint256,address,uint256,uint256,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[],address,address))";

export const L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE =
  "initialize(uint256,uint256,address,(address,bytes32)[],(uint8,bytes32)[],(uint8,bytes32)[])";

export const EMPTY_INITIALIZE_SIGNATURE = "initialize()";
