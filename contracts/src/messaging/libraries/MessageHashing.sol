// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

/**
 * @title Library to hash cross-chain messages.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
library MessageHashing {
  /**
   * @notice Hashes messages using assembly for efficiency.
   * @dev Adding 0xc0 is to indicate the calldata offset relative to the memory being added to.
   * @dev If the calldata is not modulus 32, the extra bit needs to be added on at the end else the hash is wrong.
   * @param _from The from address.
   * @param _to The to address.
   * @param _fee The fee paid for delivery.
   * @param _valueSent The value to be sent when delivering.
   * @param _messageNumber The unique message number.
   * @param _calldata The calldata to be passed to the destination address.
   */
  function _hashMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _valueSent,
    uint256 _messageNumber,
    bytes calldata _calldata
  ) internal pure returns (bytes32 messageHash) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _from)
      mstore(add(mPtr, 0x20), _to)
      mstore(add(mPtr, 0x40), _fee)
      mstore(add(mPtr, 0x60), _valueSent)
      mstore(add(mPtr, 0x80), _messageNumber)
      mstore(add(mPtr, 0xa0), 0xc0)
      mstore(add(mPtr, 0xc0), _calldata.length)
      let rem := mod(_calldata.length, 0x20)
      let extra := 0
      if iszero(iszero(rem)) {
        extra := sub(0x20, rem)
      }

      calldatacopy(add(mPtr, 0xe0), _calldata.offset, _calldata.length)
      messageHash := keccak256(mPtr, add(0xe0, add(_calldata.length, extra)))
    }
  }

  /**
   * @notice Hashes messages with empty calldata using assembly for efficiency.
   * @dev Adding 0xc0 is to indicate the calldata offset relative to the memory being added to.
   * @param _from The from address.
   * @param _to The to address.
   * @param _fee The fee paid for delivery.
   * @param _valueSent The value to be sent when delivering.
   * @param _messageNumber The unique message number.
   */
  function _hashMessageWithEmptyCalldata(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _valueSent,
    uint256 _messageNumber
  ) internal pure returns (bytes32 messageHash) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _from)
      mstore(add(mPtr, 0x20), _to)
      mstore(add(mPtr, 0x40), _fee)
      mstore(add(mPtr, 0x60), _valueSent)
      mstore(add(mPtr, 0x80), _messageNumber)
      mstore(add(mPtr, 0xa0), 0xc0)
      mstore(add(mPtr, 0xc0), 0x00)
      messageHash := keccak256(mPtr, 0xe0)
    }
  }
}
