// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { IL1LineaTokenBurner } from "./interfaces/IL1LineaTokenBurner.sol";
import { IL1MessageService } from "../messaging/l1/interfaces/IL1MessageService.sol";
import { IL1MessageManager } from "../messaging/l1/interfaces/IL1MessageManager.sol";
import { IGenericErrors } from "../interfaces/IGenericErrors.sol";
import { IL1LineaToken } from "./interfaces/IL1LineaToken.sol";

/**
 * @title L1 Linea Token Burner Contract.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract L1LineaTokenBurner is IL1LineaTokenBurner, IGenericErrors {
  /// @notice Address of the LINEA token contract
  address public immutable LINEA_TOKEN;
  /// @notice Address of the MessageService contract
  address public immutable MESSAGE_SERVICE;

  constructor(address _messageService, address _lineaToken) {
    require(_messageService != address(0), ZeroAddressNotAllowed());
    require(_lineaToken != address(0), ZeroAddressNotAllowed());

    MESSAGE_SERVICE = _messageService;
    LINEA_TOKEN = _lineaToken;

    emit L1LineaTokenBurnerInitialized(_messageService, _lineaToken);
  }

  /**
   * @notice Claims a message with proof and burns the LINEA tokens held by this contract.
   * @dev This is expected to be permissionless, allowing anyone to trigger the burn.
   * @param _params The parameters required to claim the message with proof.
   */
  function claimMessageWithProof(IL1MessageService.ClaimMessageWithProofParams calldata _params) external {
    if (!IL1MessageManager(MESSAGE_SERVICE).isMessageClaimed(_params.messageNumber)) {
      IL1MessageService(MESSAGE_SERVICE).claimMessageWithProof(_params);
    }

    uint256 balance = IL1LineaToken(LINEA_TOKEN).balanceOf(address(this));
    require(balance > 0, NoTokensToBurn());

    IL1LineaToken(LINEA_TOKEN).burn(balance);
  }
}
