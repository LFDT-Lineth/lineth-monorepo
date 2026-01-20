// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.33;

import { ValidatorContainerProofVerifier } from "../../../yield/libs/ValidatorContainerProofVerifier.sol";
import { GIndex, pack } from "../../../yield/libs/vendor/lido/GIndex.sol";
import { IValidatorContainerProofVerifier } from "../../../yield/interfaces/IValidatorContainerProofVerifier.sol";

contract TestValidatorContainerProofVerifier is ValidatorContainerProofVerifier {
  constructor(
    address _admin,
    GIndex _gIFirstValidator,
    GIndex _gIPendingPartialWithdrawalsRoot
  ) ValidatorContainerProofVerifier(_admin, _gIFirstValidator, _gIPendingPartialWithdrawalsRoot) {}

  function verifySlot(uint64 _slot, uint64 _proposerIndex, bytes32[] calldata _proof) external view {
    _verifySlot(_slot, _proposerIndex, _proof);
  }

  function validateActivationEpoch(uint64 _slot, uint64 _activationEpoch) external view {
    _validateActivationEpoch(_slot, _activationEpoch);
  }

  function getValidatorGI(uint256 _offset) external view returns (GIndex) {
    return _getValidatorGI(_offset);
  }

  function getValidatorGIInLeftSubtree(uint256 _offset) external view returns (GIndex) {
    GIndex validatorGi = _getValidatorGI(_offset);
    uint256 validatorGiIndex = validatorGi.index();
    uint8 pow = validatorGi.pow();
    // GI_FIRST_VALIDATOR is in depth 47, so need to get offset from 2**47
    uint256 offsetFromLeft = validatorGiIndex - 2 ** 47;
    uint256 gIFirstValidatorIndexInLeftSubtree = 2 ** 46 + offsetFromLeft;
    GIndex gIFirstValidatorInLeftSubtree = pack(gIFirstValidatorIndexInLeftSubtree, pow - 1);
    return gIFirstValidatorInLeftSubtree;
  }

  function getParentBlockRoot(uint64 _childBlockTimestamp) external view returns (bytes32) {
    return _getParentBlockRoot(_childBlockTimestamp);
  }
}
