import { parseEther } from "viem";

export const INITIAL_WITHDRAW_LIMIT = parseEther("5");
export const MESSAGE_VALUE_1ETH = parseEther("1");
export const ZERO_VALUE = 0;
export const MESSAGE_FEE = parseEther("0.05");
export const LOW_NO_REFUND_MESSAGE_FEE = parseEther("0.00001");
export const MINIMUM_FEE = parseEther("0.1");
export const MAX_GAS_LIMIT = 16_777_216;
