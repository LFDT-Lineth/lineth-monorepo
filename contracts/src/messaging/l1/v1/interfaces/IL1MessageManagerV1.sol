// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.26;

/**
 * @title L1 Message manager V1 interface for pre-existing errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IL1MessageManagerV1 {
  /**
   * @dev Thrown when the message has already been claimed.
   */
  error MessageDoesNotExistOrHasAlreadyBeenClaimed(bytes32 messageHash);
}
