// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title L1 Message manager V1 interface for pre-existing errors.
 * @author Consensys Incorporated
 * @custom:security-contact security-report@linea.build
 */
interface IL1MessageManagerV1 {
  /**
   * @dev Thrown when the message has already been claimed.
   */
  error MessageDoesNotExistOrHasAlreadyBeenClaimed(bytes32 messageHash);
}
