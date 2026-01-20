// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { YieldManagerStorageLayout } from "../../../yield/YieldManagerStorageLayout.sol";
import { IYieldProvider } from "../../../yield/interfaces/IYieldProvider.sol";
import { ProgressOssificationResult, YieldProviderRegistration } from "../../../yield/interfaces/YieldTypes.sol";

abstract contract MockYieldProviderStorageLayout {
  /// @dev keccak256(abi.encode(uint256(keccak256("linea.storage.MockYieldProviderStorage")) - 1)) & ~bytes32(uint256(0xff))
  bytes32 private constant MockYieldProviderStorageLocation =
    0x6872667b28c0553451b98c9cc79a262d1e6603e3af2375dec0299e2db861a700;

  struct TestYieldManagerStorage {
    mapping(address => MockYieldProviderStorage) _mockYieldProviderStorage;
  }

  struct MockYieldProviderStorage {
    uint256 withdrawableValueReturnVal;
    uint256 reportYieldReturnVal_NewReportedYield;
    uint256 reportYieldReturnVal_OutstandingNegativeYield;
    uint256 payLSTPrincipalReturnVal;
    uint256 unstakePermissionlessReturnVal;
    ProgressOssificationResult progressPendingOssificationReturnVal;
    YieldProviderRegistration initializeVendorContractsReturnVal;
    address mockWithdrawTarget;
    bool isInitiateOssificationReverting;
  }

  function _getMockYieldProviderStorage(
    address _yieldProvider
  ) internal view returns (MockYieldProviderStorage storage) {
    TestYieldManagerStorage storage $;
    assembly {
      $.slot := MockYieldProviderStorageLocation
    }
    return $._mockYieldProviderStorage[_yieldProvider];
  }

  /*//////////////////////////////////////////////////////////////
                            MOCK HELPERS
  //////////////////////////////////////////////////////////////*/

  function setMockWithdrawTarget(address _yieldProvider, address _val) external {
    _getMockYieldProviderStorage(_yieldProvider).mockWithdrawTarget = _val;
  }

  function setWithdrawableValueReturnVal(address _yieldProvider, uint256 _val) external {
    _getMockYieldProviderStorage(_yieldProvider).withdrawableValueReturnVal = _val;
  }

  function setReportYieldReturnVal_NewReportedYield(address _yieldProvider, uint256 _val) external {
    _getMockYieldProviderStorage(_yieldProvider).reportYieldReturnVal_NewReportedYield = _val;
  }

  function setReportYieldReturnVal_OutstandingNegativeYield(address _yieldProvider, uint256 _val) external {
    _getMockYieldProviderStorage(_yieldProvider).reportYieldReturnVal_OutstandingNegativeYield = _val;
  }

  function setPayLSTPrincipalReturnVal(address _yieldProvider, uint256 _val) external {
    _getMockYieldProviderStorage(_yieldProvider).payLSTPrincipalReturnVal = _val;
  }

  function setUnstakePermissionlessReturnVal(address _yieldProvider, uint256 _val) external {
    _getMockYieldProviderStorage(_yieldProvider).unstakePermissionlessReturnVal = _val;
  }

  function setProgressPendingOssificationReturnVal(address _yieldProvider, ProgressOssificationResult _val) external {
    _getMockYieldProviderStorage(_yieldProvider).progressPendingOssificationReturnVal = _val;
  }

  function getMockWithdrawTarget(address _yieldProvider) public view returns (address) {
    return _getMockYieldProviderStorage(_yieldProvider).mockWithdrawTarget;
  }

  function withdrawableValueReturnVal(address _yieldProvider) public view returns (uint256) {
    return _getMockYieldProviderStorage(_yieldProvider).withdrawableValueReturnVal;
  }

  function reportYieldReturnVal_NewReportedYield(address _yieldProvider) public view returns (uint256) {
    return _getMockYieldProviderStorage(_yieldProvider).reportYieldReturnVal_NewReportedYield;
  }

  function reportYieldReturnVal_OutstandingNegativeYield(address _yieldProvider) public view returns (uint256) {
    return _getMockYieldProviderStorage(_yieldProvider).reportYieldReturnVal_OutstandingNegativeYield;
  }

  function payLSTPrincipalReturnVal(address _yieldProvider) public view returns (uint256) {
    return _getMockYieldProviderStorage(_yieldProvider).payLSTPrincipalReturnVal;
  }

  function unstakePermissionlessReturnVal(address _yieldProvider) public view returns (uint256) {
    return _getMockYieldProviderStorage(_yieldProvider).unstakePermissionlessReturnVal;
  }

  function getProgressPendingOssificationReturnVal(
    address _yieldProvider
  ) public view returns (ProgressOssificationResult) {
    return _getMockYieldProviderStorage(_yieldProvider).progressPendingOssificationReturnVal;
  }

  function getInitializeVendorContractsReturnVal() public view returns (YieldProviderRegistration memory) {
    return _getMockYieldProviderStorage(address(0)).initializeVendorContractsReturnVal;
  }

  function setInitializeVendorContractsReturnVal(YieldProviderRegistration memory registration) public {
    _getMockYieldProviderStorage(address(0)).initializeVendorContractsReturnVal = registration;
  }

  function getIsInitiateOssificationReverting(address _yieldProvider) public view returns (bool) {
    return _getMockYieldProviderStorage(_yieldProvider).isInitiateOssificationReverting;
  }

  function setIsInitiateOssificationReverting(address _yieldProvider, bool val) external {
    _getMockYieldProviderStorage(_yieldProvider).isInitiateOssificationReverting = val;
  }
}
