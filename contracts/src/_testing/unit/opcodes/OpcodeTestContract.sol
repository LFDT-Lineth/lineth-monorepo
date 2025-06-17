// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

/// @dev Not intended for mainnet or testnet deployment, only for local testing
contract OpcodeTestContract {
  /// @dev erc7201:opcodeTestContract.main
  struct MainStorage {
    uint256 gasLimit;
  }

  // keccak256(abi.encode(uint256(keccak256("opcodeTestContract.main")) - 1)) & ~bytes32(uint256(0xff))
  bytes32 private constant MAIN_STORAGE_SLOT = 0xb69ece048aea1886497badfc9449787df15ad9606ca8687d17308088ee555100;

  function _getMainStorage() private pure returns (MainStorage storage $) {
    assembly {
      $.slot := MAIN_STORAGE_SLOT
    }
  }

  function getGasLimit() external view returns (uint256) {
    MainStorage storage $ = _getMainStorage();
    return $.gasLimit;
  }

  function setGasLimit() external {
    uint256 gasLimit;
    assembly {
      gasLimit := gaslimit()
    }
    MainStorage storage $ = _getMainStorage();
    $.gasLimit = gasLimit;
  }
}
