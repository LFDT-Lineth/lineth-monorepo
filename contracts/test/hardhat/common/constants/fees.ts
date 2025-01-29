import { ethers } from "hardhat";

export const INITIAL_WITHDRAW_LIMIT = ethers.parseEther("5");
export const MESSAGE_VALUE_1ETH = ethers.parseEther("1");
export const ZERO_VALUE = 0;
export const MESSAGE_FEE = ethers.parseEther("0.05");
export const LOW_NO_REFUND_MESSAGE_FEE = ethers.parseEther("0.00001");
export const MINIMUM_FEE = ethers.parseEther("0.1");
export const DEFAULT_MESSAGE_NONCE = ethers.parseEther("123456789");
