import { ethers } from "ethers";

const from = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"; // Replace with your deployer address
const nonce = 3; // Replace with the actual nonce
const computedAddress = ethers.getCreateAddress({ from, nonce });

console.log("Computed Address:", computedAddress);
