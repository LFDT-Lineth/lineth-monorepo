// Current fast transfer route fee for mainnet
export const CCTP_TRANSFER_MAX_FEE_FALLBACK = 100n;
// 1000 Fast transfer, 2000 Standard transfer
export const CCTP_MIN_FINALITY_THRESHOLD = 1000;
// https://developers.circle.com/stablecoins/message-format, add 2 for '0x' prefix
export const CCTP_V2_MESSAGE_HEADER_LENGTH = 298;
export const CCTP_V2_EXPIRATION_BLOCK_OFFSET = 2 + 344 * 2;
export const CCTP_V2_EXPIRATION_BLOCK_LENGTH = 64;
