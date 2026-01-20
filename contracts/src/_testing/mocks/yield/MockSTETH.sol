// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IStETH } from "../../../yield/interfaces/vendor/lido/IStETH.sol";

contract MockSTETH is IStETH {
  uint256 private pooledEthBySharesRoundUpReturn;
  uint256 private sharesByPooledEthReturn;

  function setPooledEthBySharesRoundUpReturn(uint256 _val) external {
    pooledEthBySharesRoundUpReturn = _val;
  }

  function setSharesByPooledEthReturn(uint256 _val) external {
    sharesByPooledEthReturn = _val;
  }

  function getPooledEthBySharesRoundUp(uint256 _sharesAmount) external view override returns (uint256 etherAmount) {
    return pooledEthBySharesRoundUpReturn;
  }

  function getSharesByPooledEth(uint256 _ethAmount) external view override returns (uint256) {
    return sharesByPooledEthReturn;
  }
}
