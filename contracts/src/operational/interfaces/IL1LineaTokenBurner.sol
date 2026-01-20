// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { IL1MessageService } from "../../messaging/l1/interfaces/IL1MessageService.sol";

/**
 * @title Interface for the L1 Linea Token Burner Contract.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IL1LineaTokenBurner {
  /**
   * @notice Error thrown when there are no tokens to burn.
   */
  error NoTokensToBurn();

  /**
   * @notice Emitted when the L1 Linea Token Burner is initialized.
   * @param messageService The address of the Message Service contract.
   * @param lineaToken The address of the LINEA token contract.
   */
  event L1LineaTokenBurnerInitialized(address messageService, address lineaToken);

  /**
   * @notice Claims a message with proof and burns the LINEA tokens held by this contract.
   * @param _params The parameters required to claim the message with proof.
   */
  function claimMessageWithProof(IL1MessageService.ClaimMessageWithProofParams calldata _params) external;
}
