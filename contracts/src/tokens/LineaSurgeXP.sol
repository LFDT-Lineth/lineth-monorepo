// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title ERC-20 contract for Linea Surge XP (LXP-L) tokens.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LineaSurgeXP is ERC20, AccessControl {
  error TokenIsSoulBound();
  error CallerIsNotContract();
  error ZeroAddressNotAllowed();

  bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
  bytes32 public constant TRANSFER_ROLE = keccak256("TRANSFER_ROLE");

  constructor(address admin, address minter, address[] memory _initialTransferers) ERC20("Linea Surge XP", "LXP-L") {
    if (admin == address(0) || minter == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    for (uint256 i; i < _initialTransferers.length; i++) {
      if (_initialTransferers[i] == address(0)) {
        revert ZeroAddressNotAllowed();
      }
      _grantRole(TRANSFER_ROLE, _initialTransferers[i]);
    }

    _grantRole(DEFAULT_ADMIN_ROLE, admin);
    _grantRole(MINTER_ROLE, minter);
  }

  /**
   * @notice Mints a single token amount for a single recipient.
   * @param _to The address receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function mint(address _to, uint256 _amount) external onlyRole(MINTER_ROLE) {
    _mint(_to, _amount);
  }

  /**
   * @notice Mints a single token amount for a multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMint(address[] calldata _to, uint256 _amount) external onlyRole(MINTER_ROLE) {
    uint256 addressLength = _to.length;

    for (uint256 i; i < addressLength; ) {
      unchecked {
        _mint(_to[i], _amount);
        ++i;
      }
    }
  }

  /**
   * @notice Mints a 1:1 amounts for multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amounts The amounts of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMintMultiple(address[] calldata _to, uint256[] calldata _amounts) external onlyRole(MINTER_ROLE) {
    require(_to.length == _amounts.length, "Array lengths do not match");

    uint256 addressLength = _to.length;
    for (uint256 i; i < addressLength; ) {
      unchecked {
        _mint(_to[i], _amounts[i]);
        ++i;
      }
    }
  }

  /**
   * @notice Validates allowed role and caller being contract before calling base transfer.
   * @dev Only contracts with TRANSFER_ROLE are allowed to call transfer.
   * @param _address The recipient address.
   * @param _amount The amount to transfer.
   */
  function transfer(
    address _address,
    uint256 _amount
  ) public virtual override onlyRole(TRANSFER_ROLE) onlyContractCaller returns (bool) {
    return super.transfer(_address, _amount);
  }

  /**
   * @notice Overrides and reverts base functions for transferFrom.
   */
  function transferFrom(address, address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }

  /**
   * @notice Overrides and reverts base functions for approve.
   */
  function approve(address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }

  /**
   * @notice Overrides and reverts base functions for increaseAllowance.
   */
  function increaseAllowance(address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }

  /**
   * @notice Overrides and reverts base functions for decreaseAllowance.
   */
  function decreaseAllowance(address, uint256) public virtual override returns (bool) {
    revert TokenIsSoulBound();
  }

  modifier onlyContractCaller() {
    bool isCallerContract;

    assembly {
      isCallerContract := iszero(iszero(extcodesize(caller())))
    }

    if (!isCallerContract) {
      revert CallerIsNotContract();
    }

    _;
  }
}
