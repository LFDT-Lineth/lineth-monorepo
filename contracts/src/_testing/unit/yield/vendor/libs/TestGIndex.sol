// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.25;

import { GIndex, pack as gindexPack, unwrap as gindexUnwrap, index as gindexIndex, width as gindexWidth, concat as gindexConcat, shr as gindexShr, shl as gindexShl, pow as gindexPow, fls as gindexFls, isRoot as gindexIsRoot } from "../../../../../yield/libs/vendor/lido/GIndex.sol";

contract TestGIndex {
  function wrap(bytes32 value) external pure returns (GIndex) {
    return GIndex.wrap(value);
  }

  function unwrap(GIndex gIndex) external pure returns (bytes32) {
    return gindexUnwrap(gIndex);
  }

  function pack(uint256 gI, uint8 p) external pure returns (GIndex) {
    return gindexPack(gI, p);
  }

  function isRoot(GIndex gIndex) external pure returns (bool) {
    return gindexIsRoot(gIndex);
  }

  function index(GIndex gIndex) external pure returns (uint256) {
    return gindexIndex(gIndex);
  }

  function width(GIndex gIndex) external pure returns (uint256) {
    return gindexWidth(gIndex);
  }

  function concat(GIndex lhs, GIndex rhs) external pure returns (GIndex) {
    return gindexConcat(lhs, rhs);
  }

  function shr(GIndex self, uint256 n) external pure returns (GIndex) {
    return gindexShr(self, n);
  }

  function shl(GIndex self, uint256 n) external pure returns (GIndex) {
    return gindexShl(self, n);
  }

  function pow(GIndex gIndex) external pure returns (uint8) {
    return gindexPow(gIndex);
  }

  function fls(uint256 x) external pure returns (uint256) {
    return gindexFls(x);
  }
}
