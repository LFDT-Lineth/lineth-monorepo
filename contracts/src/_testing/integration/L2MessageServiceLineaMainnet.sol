// SPDX-License-Identifier: AGPL-3.0
// Sources flattened with hardhat v2.17.0 https://hardhat.org

// File @openzeppelin/contracts-upgradeable/access/IAccessControlUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts v4.4.1 (access/IAccessControl.sol)

pragma solidity ^0.8.0;

/**
 * @dev External interface of AccessControl declared to support ERC165 detection.
 */
interface IAccessControlUpgradeable {
  /**
   * @dev Emitted when `newAdminRole` is set as ``role``'s admin role, replacing `previousAdminRole`
   *
   * `DEFAULT_ADMIN_ROLE` is the starting admin for all roles, despite
   * {RoleAdminChanged} not being emitted signaling this.
   *
   * _Available since v3.1._
   */
  event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole);

  /**
   * @dev Emitted when `account` is granted `role`.
   *
   * `sender` is the account that originated the contract call, an admin role
   * bearer except when using {AccessControl-_setupRole}.
   */
  event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender);

  /**
   * @dev Emitted when `account` is revoked `role`.
   *
   * `sender` is the account that originated the contract call:
   *   - if using `revokeRole`, it is the admin role bearer
   *   - if using `renounceRole`, it is the role bearer (i.e. `account`)
   */
  event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender);

  /**
   * @dev Returns `true` if `account` has been granted `role`.
   */
  function hasRole(bytes32 role, address account) external view returns (bool);

  /**
   * @dev Returns the admin role that controls `role`. See {grantRole} and
   * {revokeRole}.
   *
   * To change a role's admin, use {AccessControl-_setRoleAdmin}.
   */
  function getRoleAdmin(bytes32 role) external view returns (bytes32);

  /**
   * @dev Grants `role` to `account`.
   *
   * If `account` had not been already granted `role`, emits a {RoleGranted}
   * event.
   *
   * Requirements:
   *
   * - the caller must have ``role``'s admin role.
   */
  function grantRole(bytes32 role, address account) external;

  /**
   * @dev Revokes `role` from `account`.
   *
   * If `account` had been granted `role`, emits a {RoleRevoked} event.
   *
   * Requirements:
   *
   * - the caller must have ``role``'s admin role.
   */
  function revokeRole(bytes32 role, address account) external;

  /**
   * @dev Revokes `role` from the calling account.
   *
   * Roles are often managed via {grantRole} and {revokeRole}: this function's
   * purpose is to provide a mechanism for accounts to lose their privileges
   * if they are compromised (such as when a trusted device is misplaced).
   *
   * If the calling account had been granted `role`, emits a {RoleRevoked}
   * event.
   *
   * Requirements:
   *
   * - the caller must be `account`.
   */
  function renounceRole(bytes32 role, address account) external;
}

// File @openzeppelin/contracts-upgradeable/utils/AddressUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.9.0) (utils/Address.sol)

pragma solidity ^0.8.1;

/**
 * @dev Collection of functions related to the address type
 */
library AddressUpgradeable {
  /**
   * @dev Returns true if `account` is a contract.
   *
   * [IMPORTANT]
   * ====
   * It is unsafe to assume that an address for which this function returns
   * false is an externally-owned account (EOA) and not a contract.
   *
   * Among others, `isContract` will return false for the following
   * types of addresses:
   *
   *  - an externally-owned account
   *  - a contract in construction
   *  - an address where a contract will be created
   *  - an address where a contract lived, but was destroyed
   *
   * Furthermore, `isContract` will also return true if the target contract within
   * the same transaction is already scheduled for destruction by `SELFDESTRUCT`,
   * which only has an effect at the end of a transaction.
   * ====
   *
   * [IMPORTANT]
   * ====
   * You shouldn't rely on `isContract` to protect against flash loan attacks!
   *
   * Preventing calls from contracts is highly discouraged. It breaks composability, breaks support for smart wallets
   * like Gnosis Safe, and does not provide security since it can be circumvented by calling from a contract
   * constructor.
   * ====
   */
  function isContract(address account) internal view returns (bool) {
    // This method relies on extcodesize/address.code.length, which returns 0
    // for contracts in construction, since the code is only stored at the end
    // of the constructor execution.

    return account.code.length > 0;
  }

  /**
   * @dev Replacement for Solidity's `transfer`: sends `amount` wei to
   * `recipient`, forwarding all available gas and reverting on errors.
   *
   * https://eips.ethereum.org/EIPS/eip-1884[EIP1884] increases the gas cost
   * of certain opcodes, possibly making contracts go over the 2300 gas limit
   * imposed by `transfer`, making them unable to receive funds via
   * `transfer`. {sendValue} removes this limitation.
   *
   * https://consensys.io/diligence/blog/2019/09/stop-using-soliditys-transfer-now/[Learn more].
   *
   * IMPORTANT: because control is transferred to `recipient`, care must be
   * taken to not create reentrancy vulnerabilities. Consider using
   * {ReentrancyGuard} or the
   * https://solidity.readthedocs.io/en/v0.8.0/security-considerations.html#use-the-checks-effects-interactions-pattern[checks-effects-interactions pattern].
   */
  function sendValue(address payable recipient, uint256 amount) internal {
    require(address(this).balance >= amount, "Address: insufficient balance");

    (bool success, ) = recipient.call{ value: amount }("");
    require(success, "Address: unable to send value, recipient may have reverted");
  }

  /**
   * @dev Performs a Solidity function call using a low level `call`. A
   * plain `call` is an unsafe replacement for a function call: use this
   * function instead.
   *
   * If `target` reverts with a revert reason, it is bubbled up by this
   * function (like regular Solidity function calls).
   *
   * Returns the raw returned data. To convert to the expected return value,
   * use https://solidity.readthedocs.io/en/latest/units-and-global-variables.html?highlight=abi.decode#abi-encoding-and-decoding-functions[`abi.decode`].
   *
   * Requirements:
   *
   * - `target` must be a contract.
   * - calling `target` with `data` must not revert.
   *
   * _Available since v3.1._
   */
  function functionCall(address target, bytes memory data) internal returns (bytes memory) {
    return functionCallWithValue(target, data, 0, "Address: low-level call failed");
  }

  /**
   * @dev Same as {xref-Address-functionCall-address-bytes-}[`functionCall`], but with
   * `errorMessage` as a fallback revert reason when `target` reverts.
   *
   * _Available since v3.1._
   */
  function functionCall(address target, bytes memory data, string memory errorMessage) internal returns (bytes memory) {
    return functionCallWithValue(target, data, 0, errorMessage);
  }

  /**
   * @dev Same as {xref-Address-functionCall-address-bytes-}[`functionCall`],
   * but also transferring `value` wei to `target`.
   *
   * Requirements:
   *
   * - the calling contract must have an ETH balance of at least `value`.
   * - the called Solidity function must be `payable`.
   *
   * _Available since v3.1._
   */
  function functionCallWithValue(address target, bytes memory data, uint256 value) internal returns (bytes memory) {
    return functionCallWithValue(target, data, value, "Address: low-level call with value failed");
  }

  /**
   * @dev Same as {xref-Address-functionCallWithValue-address-bytes-uint256-}[`functionCallWithValue`], but
   * with `errorMessage` as a fallback revert reason when `target` reverts.
   *
   * _Available since v3.1._
   */
  function functionCallWithValue(
    address target,
    bytes memory data,
    uint256 value,
    string memory errorMessage
  ) internal returns (bytes memory) {
    require(address(this).balance >= value, "Address: insufficient balance for call");
    (bool success, bytes memory returndata) = target.call{ value: value }(data);
    return verifyCallResultFromTarget(target, success, returndata, errorMessage);
  }

  /**
   * @dev Same as {xref-Address-functionCall-address-bytes-}[`functionCall`],
   * but performing a static call.
   *
   * _Available since v3.3._
   */
  function functionStaticCall(address target, bytes memory data) internal view returns (bytes memory) {
    return functionStaticCall(target, data, "Address: low-level static call failed");
  }

  /**
   * @dev Same as {xref-Address-functionCall-address-bytes-string-}[`functionCall`],
   * but performing a static call.
   *
   * _Available since v3.3._
   */
  function functionStaticCall(
    address target,
    bytes memory data,
    string memory errorMessage
  ) internal view returns (bytes memory) {
    (bool success, bytes memory returndata) = target.staticcall(data);
    return verifyCallResultFromTarget(target, success, returndata, errorMessage);
  }

  /**
   * @dev Same as {xref-Address-functionCall-address-bytes-}[`functionCall`],
   * but performing a delegate call.
   *
   * _Available since v3.4._
   */
  function functionDelegateCall(address target, bytes memory data) internal returns (bytes memory) {
    return functionDelegateCall(target, data, "Address: low-level delegate call failed");
  }

  /**
   * @dev Same as {xref-Address-functionCall-address-bytes-string-}[`functionCall`],
   * but performing a delegate call.
   *
   * _Available since v3.4._
   */
  function functionDelegateCall(
    address target,
    bytes memory data,
    string memory errorMessage
  ) internal returns (bytes memory) {
    (bool success, bytes memory returndata) = target.delegatecall(data);
    return verifyCallResultFromTarget(target, success, returndata, errorMessage);
  }

  /**
   * @dev Tool to verify that a low level call to smart-contract was successful, and revert (either by bubbling
   * the revert reason or using the provided one) in case of unsuccessful call or if target was not a contract.
   *
   * _Available since v4.8._
   */
  function verifyCallResultFromTarget(
    address target,
    bool success,
    bytes memory returndata,
    string memory errorMessage
  ) internal view returns (bytes memory) {
    if (success) {
      if (returndata.length == 0) {
        // only check isContract if the call was successful and the return data is empty
        // otherwise we already know that it was a contract
        require(isContract(target), "Address: call to non-contract");
      }
      return returndata;
    } else {
      _revert(returndata, errorMessage);
    }
  }

  /**
   * @dev Tool to verify that a low level call was successful, and revert if it wasn't, either by bubbling the
   * revert reason or using the provided one.
   *
   * _Available since v4.3._
   */
  function verifyCallResult(
    bool success,
    bytes memory returndata,
    string memory errorMessage
  ) internal pure returns (bytes memory) {
    if (success) {
      return returndata;
    } else {
      _revert(returndata, errorMessage);
    }
  }

  function _revert(bytes memory returndata, string memory errorMessage) private pure {
    // Look for revert reason and bubble it up if present
    if (returndata.length > 0) {
      // The easiest way to bubble the revert reason is using memory via assembly
      /// @solidity memory-safe-assembly
      assembly {
        let returndata_size := mload(returndata)
        revert(add(32, returndata), returndata_size)
      }
    } else {
      revert(errorMessage);
    }
  }
}

// File @openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.9.0) (proxy/utils/Initializable.sol)

pragma solidity ^0.8.2;

/**
 * @dev This is a base contract to aid in writing upgradeable contracts, or any kind of contract that will be deployed
 * behind a proxy. Since proxied contracts do not make use of a constructor, it's common to move constructor logic to an
 * external initializer function, usually called `initialize`. It then becomes necessary to protect this initializer
 * function so it can only be called once. The {initializer} modifier provided by this contract will have this effect.
 *
 * The initialization functions use a version number. Once a version number is used, it is consumed and cannot be
 * reused. This mechanism prevents re-execution of each "step" but allows the creation of new initialization steps in
 * case an upgrade adds a module that needs to be initialized.
 *
 * For example:
 *
 * [.hljs-theme-light.nopadding]
 * ```solidity
 * contract MyToken is ERC20Upgradeable {
 *     function initialize() initializer public {
 *         __ERC20_init("MyToken", "MTK");
 *     }
 * }
 *
 * contract MyTokenV2 is MyToken, ERC20PermitUpgradeable {
 *     function initializeV2() reinitializer(2) public {
 *         __ERC20Permit_init("MyToken");
 *     }
 * }
 * ```
 *
 * TIP: To avoid leaving the proxy in an uninitialized state, the initializer function should be called as early as
 * possible by providing the encoded function call as the `_data` argument to {ERC1967Proxy-constructor}.
 *
 * CAUTION: When used with inheritance, manual care must be taken to not invoke a parent initializer twice, or to ensure
 * that all initializers are idempotent. This is not verified automatically as constructors are by Solidity.
 *
 * [CAUTION]
 * ====
 * Avoid leaving a contract uninitialized.
 *
 * An uninitialized contract can be taken over by an attacker. This applies to both a proxy and its implementation
 * contract, which may impact the proxy. To prevent the implementation contract from being used, you should invoke
 * the {_disableInitializers} function in the constructor to automatically lock it when it is deployed:
 *
 * [.hljs-theme-light.nopadding]
 * ```
 * /// @custom:oz-upgrades-unsafe-allow constructor
 * constructor() {
 *     _disableInitializers();
 * }
 * ```
 * ====
 */
abstract contract Initializable {
  /**
   * @dev Indicates that the contract has been initialized.
   * @custom:oz-retyped-from bool
   */
  uint8 private _initialized;

  /**
   * @dev Indicates that the contract is in the process of being initialized.
   */
  bool private _initializing;

  /**
   * @dev Triggered when the contract has been initialized or reinitialized.
   */
  event Initialized(uint8 version);

  /**
   * @dev A modifier that defines a protected initializer function that can be invoked at most once. In its scope,
   * `onlyInitializing` functions can be used to initialize parent contracts.
   *
   * Similar to `reinitializer(1)`, except that functions marked with `initializer` can be nested in the context of a
   * constructor.
   *
   * Emits an {Initialized} event.
   */
  modifier initializer() {
    bool isTopLevelCall = !_initializing;
    require(
      (isTopLevelCall && _initialized < 1) || (!AddressUpgradeable.isContract(address(this)) && _initialized == 1),
      "Initializable: contract is already initialized"
    );
    _initialized = 1;
    if (isTopLevelCall) {
      _initializing = true;
    }
    _;
    if (isTopLevelCall) {
      _initializing = false;
      emit Initialized(1);
    }
  }

  /**
   * @dev A modifier that defines a protected reinitializer function that can be invoked at most once, and only if the
   * contract hasn't been initialized to a greater version before. In its scope, `onlyInitializing` functions can be
   * used to initialize parent contracts.
   *
   * A reinitializer may be used after the original initialization step. This is essential to configure modules that
   * are added through upgrades and that require initialization.
   *
   * When `version` is 1, this modifier is similar to `initializer`, except that functions marked with `reinitializer`
   * cannot be nested. If one is invoked in the context of another, execution will revert.
   *
   * Note that versions can jump in increments greater than 1; this implies that if multiple reinitializers coexist in
   * a contract, executing them in the right order is up to the developer or operator.
   *
   * WARNING: setting the version to 255 will prevent any future reinitialization.
   *
   * Emits an {Initialized} event.
   */
  modifier reinitializer(uint8 version) {
    require(!_initializing && _initialized < version, "Initializable: contract is already initialized");
    _initialized = version;
    _initializing = true;
    _;
    _initializing = false;
    emit Initialized(version);
  }

  /**
   * @dev Modifier to protect an initialization function so that it can only be invoked by functions with the
   * {initializer} and {reinitializer} modifiers, directly or indirectly.
   */
  modifier onlyInitializing() {
    require(_initializing, "Initializable: contract is not initializing");
    _;
  }

  /**
   * @dev Locks the contract, preventing any future reinitialization. This cannot be part of an initializer call.
   * Calling this in the constructor of a contract will prevent that contract from being initialized or reinitialized
   * to any version. It is recommended to use this to lock implementation contracts that are designed to be called
   * through proxies.
   *
   * Emits an {Initialized} event the first time it is successfully executed.
   */
  function _disableInitializers() internal virtual {
    require(!_initializing, "Initializable: contract is initializing");
    if (_initialized != type(uint8).max) {
      _initialized = type(uint8).max;
      emit Initialized(type(uint8).max);
    }
  }

  /**
   * @dev Returns the highest version that has been initialized. See {reinitializer}.
   */
  function _getInitializedVersion() internal view returns (uint8) {
    return _initialized;
  }

  /**
   * @dev Returns `true` if the contract is currently initializing. See {onlyInitializing}.
   */
  function _isInitializing() internal view returns (bool) {
    return _initializing;
  }
}

// File @openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts v4.4.1 (utils/Context.sol)

pragma solidity ^0.8.0;

/**
 * @dev Provides information about the current execution context, including the
 * sender of the transaction and its data. While these are generally available
 * via msg.sender and msg.data, they should not be accessed in such a direct
 * manner, since when dealing with meta-transactions the account sending and
 * paying for execution may not be the actual sender (as far as an application
 * is concerned).
 *
 * This contract is only required for intermediate, library-like contracts.
 */
abstract contract ContextUpgradeable is Initializable {
  function __Context_init() internal onlyInitializing {}

  function __Context_init_unchained() internal onlyInitializing {}

  function _msgSender() internal view virtual returns (address) {
    return msg.sender;
  }

  function _msgData() internal view virtual returns (bytes calldata) {
    return msg.data;
  }

  /**
   * @dev This empty reserved space is put in place to allow future versions to add new
   * variables without shifting down storage in the inheritance chain.
   * See https://docs.openzeppelin.com/contracts/4.x/upgradeable#storage_gaps
   */
  uint256[50] private __gap;
}

// File @openzeppelin/contracts-upgradeable/utils/introspection/IERC165Upgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts v4.4.1 (utils/introspection/IERC165.sol)

pragma solidity ^0.8.0;

/**
 * @dev Interface of the ERC165 standard, as defined in the
 * https://eips.ethereum.org/EIPS/eip-165[EIP].
 *
 * Implementers can declare support of contract interfaces, which can then be
 * queried by others ({ERC165Checker}).
 *
 * For an implementation, see {ERC165}.
 */
interface IERC165Upgradeable {
  /**
   * @dev Returns true if this contract implements the interface defined by
   * `interfaceId`. See the corresponding
   * https://eips.ethereum.org/EIPS/eip-165#how-interfaces-are-identified[EIP section]
   * to learn more about how these ids are created.
   *
   * This function call must use less than 30 000 gas.
   */
  function supportsInterface(bytes4 interfaceId) external view returns (bool);
}

// File @openzeppelin/contracts-upgradeable/utils/introspection/ERC165Upgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts v4.4.1 (utils/introspection/ERC165.sol)

pragma solidity ^0.8.0;

/**
 * @dev Implementation of the {IERC165} interface.
 *
 * Contracts that want to implement ERC165 should inherit from this contract and override {supportsInterface} to check
 * for the additional interface id that will be supported. For example:
 *
 * ```solidity
 * function supportsInterface(bytes4 interfaceId) public view virtual override returns (bool) {
 *     return interfaceId == type(MyInterface).interfaceId || super.supportsInterface(interfaceId);
 * }
 * ```
 *
 * Alternatively, {ERC165Storage} provides an easier to use but more expensive implementation.
 */
abstract contract ERC165Upgradeable is Initializable, IERC165Upgradeable {
  function __ERC165_init() internal onlyInitializing {}

  function __ERC165_init_unchained() internal onlyInitializing {}

  /**
   * @dev See {IERC165-supportsInterface}.
   */
  function supportsInterface(bytes4 interfaceId) public view virtual override returns (bool) {
    return interfaceId == type(IERC165Upgradeable).interfaceId;
  }

  /**
   * @dev This empty reserved space is put in place to allow future versions to add new
   * variables without shifting down storage in the inheritance chain.
   * See https://docs.openzeppelin.com/contracts/4.x/upgradeable#storage_gaps
   */
  uint256[50] private __gap;
}

// File @openzeppelin/contracts-upgradeable/utils/math/MathUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.9.0) (utils/math/Math.sol)

pragma solidity ^0.8.0;

/**
 * @dev Standard math utilities missing in the Solidity language.
 */
library MathUpgradeable {
  enum Rounding {
    Down, // Toward negative infinity
    Up, // Toward infinity
    Zero // Toward zero
  }

  /**
   * @dev Returns the largest of two numbers.
   */
  function max(uint256 a, uint256 b) internal pure returns (uint256) {
    return a > b ? a : b;
  }

  /**
   * @dev Returns the smallest of two numbers.
   */
  function min(uint256 a, uint256 b) internal pure returns (uint256) {
    return a < b ? a : b;
  }

  /**
   * @dev Returns the average of two numbers. The result is rounded towards
   * zero.
   */
  function average(uint256 a, uint256 b) internal pure returns (uint256) {
    // (a + b) / 2 can overflow.
    return (a & b) + (a ^ b) / 2;
  }

  /**
   * @dev Returns the ceiling of the division of two numbers.
   *
   * This differs from standard division with `/` in that it rounds up instead
   * of rounding down.
   */
  function ceilDiv(uint256 a, uint256 b) internal pure returns (uint256) {
    // (a + b - 1) / b can overflow on addition, so we distribute.
    return a == 0 ? 0 : (a - 1) / b + 1;
  }

  /**
   * @notice Calculates floor(x * y / denominator) with full precision. Throws if result overflows a uint256 or denominator == 0
   * @dev Original credit to Remco Bloemen under MIT license (https://xn--2-umb.com/21/muldiv)
   * with further edits by Uniswap Labs also under MIT license.
   */
  function mulDiv(uint256 x, uint256 y, uint256 denominator) internal pure returns (uint256 result) {
    unchecked {
      // 512-bit multiply [prod1 prod0] = x * y. Compute the product mod 2^256 and mod 2^256 - 1, then use
      // use the Chinese Remainder Theorem to reconstruct the 512 bit result. The result is stored in two 256
      // variables such that product = prod1 * 2^256 + prod0.
      uint256 prod0; // Least significant 256 bits of the product
      uint256 prod1; // Most significant 256 bits of the product
      assembly {
        let mm := mulmod(x, y, not(0))
        prod0 := mul(x, y)
        prod1 := sub(sub(mm, prod0), lt(mm, prod0))
      }

      // Handle non-overflow cases, 256 by 256 division.
      if (prod1 == 0) {
        // Solidity will revert if denominator == 0, unlike the div opcode on its own.
        // The surrounding unchecked block does not change this fact.
        // See https://docs.soliditylang.org/en/latest/control-structures.html#checked-or-unchecked-arithmetic.
        return prod0 / denominator;
      }

      // Make sure the result is less than 2^256. Also prevents denominator == 0.
      require(denominator > prod1, "Math: mulDiv overflow");

      ///////////////////////////////////////////////
      // 512 by 256 division.
      ///////////////////////////////////////////////

      // Make division exact by subtracting the remainder from [prod1 prod0].
      uint256 remainder;
      assembly {
        // Compute remainder using mulmod.
        remainder := mulmod(x, y, denominator)

        // Subtract 256 bit number from 512 bit number.
        prod1 := sub(prod1, gt(remainder, prod0))
        prod0 := sub(prod0, remainder)
      }

      // Factor powers of two out of denominator and compute largest power of two divisor of denominator. Always >= 1.
      // See https://cs.stackexchange.com/q/138556/92363.

      // Does not overflow because the denominator cannot be zero at this stage in the function.
      uint256 twos = denominator & (~denominator + 1);
      assembly {
        // Divide denominator by twos.
        denominator := div(denominator, twos)

        // Divide [prod1 prod0] by twos.
        prod0 := div(prod0, twos)

        // Flip twos such that it is 2^256 / twos. If twos is zero, then it becomes one.
        twos := add(div(sub(0, twos), twos), 1)
      }

      // Shift in bits from prod1 into prod0.
      prod0 |= prod1 * twos;

      // Invert denominator mod 2^256. Now that denominator is an odd number, it has an inverse modulo 2^256 such
      // that denominator * inv = 1 mod 2^256. Compute the inverse by starting with a seed that is correct for
      // four bits. That is, denominator * inv = 1 mod 2^4.
      uint256 inverse = (3 * denominator) ^ 2;

      // Use the Newton-Raphson iteration to improve the precision. Thanks to Hensel's lifting lemma, this also works
      // in modular arithmetic, doubling the correct bits in each step.
      inverse *= 2 - denominator * inverse; // inverse mod 2^8
      inverse *= 2 - denominator * inverse; // inverse mod 2^16
      inverse *= 2 - denominator * inverse; // inverse mod 2^32
      inverse *= 2 - denominator * inverse; // inverse mod 2^64
      inverse *= 2 - denominator * inverse; // inverse mod 2^128
      inverse *= 2 - denominator * inverse; // inverse mod 2^256

      // Because the division is now exact we can divide by multiplying with the modular inverse of denominator.
      // This will give us the correct result modulo 2^256. Since the preconditions guarantee that the outcome is
      // less than 2^256, this is the final result. We don't need to compute the high bits of the result and prod1
      // is no longer required.
      result = prod0 * inverse;
      return result;
    }
  }

  /**
   * @notice Calculates x * y / denominator with full precision, following the selected rounding direction.
   */
  function mulDiv(uint256 x, uint256 y, uint256 denominator, Rounding rounding) internal pure returns (uint256) {
    uint256 result = mulDiv(x, y, denominator);
    if (rounding == Rounding.Up && mulmod(x, y, denominator) > 0) {
      result += 1;
    }
    return result;
  }

  /**
   * @dev Returns the square root of a number. If the number is not a perfect square, the value is rounded down.
   *
   * Inspired by Henry S. Warren, Jr.'s "Hacker's Delight" (Chapter 11).
   */
  function sqrt(uint256 a) internal pure returns (uint256) {
    if (a == 0) {
      return 0;
    }

    // For our first guess, we get the biggest power of 2 which is smaller than the square root of the target.
    //
    // We know that the "msb" (most significant bit) of our target number `a` is a power of 2 such that we have
    // `msb(a) <= a < 2*msb(a)`. This value can be written `msb(a)=2**k` with `k=log2(a)`.
    //
    // This can be rewritten `2**log2(a) <= a < 2**(log2(a) + 1)`
    // → `sqrt(2**k) <= sqrt(a) < sqrt(2**(k+1))`
    // → `2**(k/2) <= sqrt(a) < 2**((k+1)/2) <= 2**(k/2 + 1)`
    //
    // Consequently, `2**(log2(a) / 2)` is a good first approximation of `sqrt(a)` with at least 1 correct bit.
    uint256 result = 1 << (log2(a) >> 1);

    // At this point `result` is an estimation with one bit of precision. We know the true value is a uint128,
    // since it is the square root of a uint256. Newton's method converges quadratically (precision doubles at
    // every iteration). We thus need at most 7 iteration to turn our partial result with one bit of precision
    // into the expected uint128 result.
    unchecked {
      result = (result + a / result) >> 1;
      result = (result + a / result) >> 1;
      result = (result + a / result) >> 1;
      result = (result + a / result) >> 1;
      result = (result + a / result) >> 1;
      result = (result + a / result) >> 1;
      result = (result + a / result) >> 1;
      return min(result, a / result);
    }
  }

  /**
   * @notice Calculates sqrt(a), following the selected rounding direction.
   */
  function sqrt(uint256 a, Rounding rounding) internal pure returns (uint256) {
    unchecked {
      uint256 result = sqrt(a);
      return result + (rounding == Rounding.Up && result * result < a ? 1 : 0);
    }
  }

  /**
   * @dev Return the log in base 2, rounded down, of a positive value.
   * Returns 0 if given 0.
   */
  function log2(uint256 value) internal pure returns (uint256) {
    uint256 result = 0;
    unchecked {
      if (value >> 128 > 0) {
        value >>= 128;
        result += 128;
      }
      if (value >> 64 > 0) {
        value >>= 64;
        result += 64;
      }
      if (value >> 32 > 0) {
        value >>= 32;
        result += 32;
      }
      if (value >> 16 > 0) {
        value >>= 16;
        result += 16;
      }
      if (value >> 8 > 0) {
        value >>= 8;
        result += 8;
      }
      if (value >> 4 > 0) {
        value >>= 4;
        result += 4;
      }
      if (value >> 2 > 0) {
        value >>= 2;
        result += 2;
      }
      if (value >> 1 > 0) {
        result += 1;
      }
    }
    return result;
  }

  /**
   * @dev Return the log in base 2, following the selected rounding direction, of a positive value.
   * Returns 0 if given 0.
   */
  function log2(uint256 value, Rounding rounding) internal pure returns (uint256) {
    unchecked {
      uint256 result = log2(value);
      return result + (rounding == Rounding.Up && 1 << result < value ? 1 : 0);
    }
  }

  /**
   * @dev Return the log in base 10, rounded down, of a positive value.
   * Returns 0 if given 0.
   */
  function log10(uint256 value) internal pure returns (uint256) {
    uint256 result = 0;
    unchecked {
      if (value >= 10 ** 64) {
        value /= 10 ** 64;
        result += 64;
      }
      if (value >= 10 ** 32) {
        value /= 10 ** 32;
        result += 32;
      }
      if (value >= 10 ** 16) {
        value /= 10 ** 16;
        result += 16;
      }
      if (value >= 10 ** 8) {
        value /= 10 ** 8;
        result += 8;
      }
      if (value >= 10 ** 4) {
        value /= 10 ** 4;
        result += 4;
      }
      if (value >= 10 ** 2) {
        value /= 10 ** 2;
        result += 2;
      }
      if (value >= 10 ** 1) {
        result += 1;
      }
    }
    return result;
  }

  /**
   * @dev Return the log in base 10, following the selected rounding direction, of a positive value.
   * Returns 0 if given 0.
   */
  function log10(uint256 value, Rounding rounding) internal pure returns (uint256) {
    unchecked {
      uint256 result = log10(value);
      return result + (rounding == Rounding.Up && 10 ** result < value ? 1 : 0);
    }
  }

  /**
   * @dev Return the log in base 256, rounded down, of a positive value.
   * Returns 0 if given 0.
   *
   * Adding one to the result gives the number of pairs of hex symbols needed to represent `value` as a hex string.
   */
  function log256(uint256 value) internal pure returns (uint256) {
    uint256 result = 0;
    unchecked {
      if (value >> 128 > 0) {
        value >>= 128;
        result += 16;
      }
      if (value >> 64 > 0) {
        value >>= 64;
        result += 8;
      }
      if (value >> 32 > 0) {
        value >>= 32;
        result += 4;
      }
      if (value >> 16 > 0) {
        value >>= 16;
        result += 2;
      }
      if (value >> 8 > 0) {
        result += 1;
      }
    }
    return result;
  }

  /**
   * @dev Return the log in base 256, following the selected rounding direction, of a positive value.
   * Returns 0 if given 0.
   */
  function log256(uint256 value, Rounding rounding) internal pure returns (uint256) {
    unchecked {
      uint256 result = log256(value);
      return result + (rounding == Rounding.Up && 1 << (result << 3) < value ? 1 : 0);
    }
  }
}

// File @openzeppelin/contracts-upgradeable/utils/math/SignedMathUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.8.0) (utils/math/SignedMath.sol)

pragma solidity ^0.8.0;

/**
 * @dev Standard signed math utilities missing in the Solidity language.
 */
library SignedMathUpgradeable {
  /**
   * @dev Returns the largest of two signed numbers.
   */
  function max(int256 a, int256 b) internal pure returns (int256) {
    return a > b ? a : b;
  }

  /**
   * @dev Returns the smallest of two signed numbers.
   */
  function min(int256 a, int256 b) internal pure returns (int256) {
    return a < b ? a : b;
  }

  /**
   * @dev Returns the average of two signed numbers without overflow.
   * The result is rounded towards zero.
   */
  function average(int256 a, int256 b) internal pure returns (int256) {
    // Formula from the book "Hacker's Delight"
    int256 x = (a & b) + ((a ^ b) >> 1);
    return x + (int256(uint256(x) >> 255) & (a ^ b));
  }

  /**
   * @dev Returns the absolute unsigned value of a signed value.
   */
  function abs(int256 n) internal pure returns (uint256) {
    unchecked {
      // must be unchecked in order to support `n = type(int256).min`
      return uint256(n >= 0 ? n : -n);
    }
  }
}

// File @openzeppelin/contracts-upgradeable/utils/StringsUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.9.0) (utils/Strings.sol)

pragma solidity ^0.8.0;

/**
 * @dev String operations.
 */
library StringsUpgradeable {
  bytes16 private constant _SYMBOLS = "0123456789abcdef";
  uint8 private constant _ADDRESS_LENGTH = 20;

  /**
   * @dev Converts a `uint256` to its ASCII `string` decimal representation.
   */
  function toString(uint256 value) internal pure returns (string memory) {
    unchecked {
      uint256 length = MathUpgradeable.log10(value) + 1;
      string memory buffer = new string(length);
      uint256 ptr;
      /// @solidity memory-safe-assembly
      assembly {
        ptr := add(buffer, add(32, length))
      }
      while (true) {
        ptr--;
        /// @solidity memory-safe-assembly
        assembly {
          mstore8(ptr, byte(mod(value, 10), _SYMBOLS))
        }
        value /= 10;
        if (value == 0) break;
      }
      return buffer;
    }
  }

  /**
   * @dev Converts a `int256` to its ASCII `string` decimal representation.
   */
  function toString(int256 value) internal pure returns (string memory) {
    return string(abi.encodePacked(value < 0 ? "-" : "", toString(SignedMathUpgradeable.abs(value))));
  }

  /**
   * @dev Converts a `uint256` to its ASCII `string` hexadecimal representation.
   */
  function toHexString(uint256 value) internal pure returns (string memory) {
    unchecked {
      return toHexString(value, MathUpgradeable.log256(value) + 1);
    }
  }

  /**
   * @dev Converts a `uint256` to its ASCII `string` hexadecimal representation with fixed length.
   */
  function toHexString(uint256 value, uint256 length) internal pure returns (string memory) {
    bytes memory buffer = new bytes(2 * length + 2);
    buffer[0] = "0";
    buffer[1] = "x";
    for (uint256 i = 2 * length + 1; i > 1; --i) {
      buffer[i] = _SYMBOLS[value & 0xf];
      value >>= 4;
    }
    require(value == 0, "Strings: hex length insufficient");
    return string(buffer);
  }

  /**
   * @dev Converts an `address` with fixed length of 20 bytes to its not checksummed ASCII `string` hexadecimal representation.
   */
  function toHexString(address addr) internal pure returns (string memory) {
    return toHexString(uint256(uint160(addr)), _ADDRESS_LENGTH);
  }

  /**
   * @dev Returns true if the two strings are equal.
   */
  function equal(string memory a, string memory b) internal pure returns (bool) {
    return keccak256(bytes(a)) == keccak256(bytes(b));
  }
}

// File @openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.9.0) (access/AccessControl.sol)

pragma solidity ^0.8.0;

/**
 * @dev Contract module that allows children to implement role-based access
 * control mechanisms. This is a lightweight version that doesn't allow enumerating role
 * members except through off-chain means by accessing the contract event logs. Some
 * applications may benefit from on-chain enumerability, for those cases see
 * {AccessControlEnumerable}.
 *
 * Roles are referred to by their `bytes32` identifier. These should be exposed
 * in the external API and be unique. The best way to achieve this is by
 * using `public constant` hash digests:
 *
 * ```solidity
 * bytes32 public constant MY_ROLE = keccak256("MY_ROLE");
 * ```
 *
 * Roles can be used to represent a set of permissions. To restrict access to a
 * function call, use {hasRole}:
 *
 * ```solidity
 * function foo() public {
 *     require(hasRole(MY_ROLE, msg.sender));
 *     ...
 * }
 * ```
 *
 * Roles can be granted and revoked dynamically via the {grantRole} and
 * {revokeRole} functions. Each role has an associated admin role, and only
 * accounts that have a role's admin role can call {grantRole} and {revokeRole}.
 *
 * By default, the admin role for all roles is `DEFAULT_ADMIN_ROLE`, which means
 * that only accounts with this role will be able to grant or revoke other
 * roles. More complex role relationships can be created by using
 * {_setRoleAdmin}.
 *
 * WARNING: The `DEFAULT_ADMIN_ROLE` is also its own admin: it has permission to
 * grant and revoke this role. Extra precautions should be taken to secure
 * accounts that have been granted it. We recommend using {AccessControlDefaultAdminRules}
 * to enforce additional security measures for this role.
 */
abstract contract AccessControlUpgradeable is
  Initializable,
  ContextUpgradeable,
  IAccessControlUpgradeable,
  ERC165Upgradeable
{
  function __AccessControl_init() internal onlyInitializing {}

  function __AccessControl_init_unchained() internal onlyInitializing {}

  struct RoleData {
    mapping(address => bool) members;
    bytes32 adminRole;
  }

  mapping(bytes32 => RoleData) private _roles;

  bytes32 public constant DEFAULT_ADMIN_ROLE = 0x00;

  /**
   * @dev Modifier that checks that an account has a specific role. Reverts
   * with a standardized message including the required role.
   *
   * The format of the revert reason is given by the following regular expression:
   *
   *  /^AccessControl: account (0x[0-9a-f]{40}) is missing role (0x[0-9a-f]{64})$/
   *
   * _Available since v4.1._
   */
  modifier onlyRole(bytes32 role) {
    _checkRole(role);
    _;
  }

  /**
   * @dev See {IERC165-supportsInterface}.
   */
  function supportsInterface(bytes4 interfaceId) public view virtual override returns (bool) {
    return interfaceId == type(IAccessControlUpgradeable).interfaceId || super.supportsInterface(interfaceId);
  }

  /**
   * @dev Returns `true` if `account` has been granted `role`.
   */
  function hasRole(bytes32 role, address account) public view virtual override returns (bool) {
    return _roles[role].members[account];
  }

  /**
   * @dev Revert with a standard message if `_msgSender()` is missing `role`.
   * Overriding this function changes the behavior of the {onlyRole} modifier.
   *
   * Format of the revert message is described in {_checkRole}.
   *
   * _Available since v4.6._
   */
  function _checkRole(bytes32 role) internal view virtual {
    _checkRole(role, _msgSender());
  }

  /**
   * @dev Revert with a standard message if `account` is missing `role`.
   *
   * The format of the revert reason is given by the following regular expression:
   *
   *  /^AccessControl: account (0x[0-9a-f]{40}) is missing role (0x[0-9a-f]{64})$/
   */
  function _checkRole(bytes32 role, address account) internal view virtual {
    if (!hasRole(role, account)) {
      revert(
        string(
          abi.encodePacked(
            "AccessControl: account ",
            StringsUpgradeable.toHexString(account),
            " is missing role ",
            StringsUpgradeable.toHexString(uint256(role), 32)
          )
        )
      );
    }
  }

  /**
   * @dev Returns the admin role that controls `role`. See {grantRole} and
   * {revokeRole}.
   *
   * To change a role's admin, use {_setRoleAdmin}.
   */
  function getRoleAdmin(bytes32 role) public view virtual override returns (bytes32) {
    return _roles[role].adminRole;
  }

  /**
   * @dev Grants `role` to `account`.
   *
   * If `account` had not been already granted `role`, emits a {RoleGranted}
   * event.
   *
   * Requirements:
   *
   * - the caller must have ``role``'s admin role.
   *
   * May emit a {RoleGranted} event.
   */
  function grantRole(bytes32 role, address account) public virtual override onlyRole(getRoleAdmin(role)) {
    _grantRole(role, account);
  }

  /**
   * @dev Revokes `role` from `account`.
   *
   * If `account` had been granted `role`, emits a {RoleRevoked} event.
   *
   * Requirements:
   *
   * - the caller must have ``role``'s admin role.
   *
   * May emit a {RoleRevoked} event.
   */
  function revokeRole(bytes32 role, address account) public virtual override onlyRole(getRoleAdmin(role)) {
    _revokeRole(role, account);
  }

  /**
   * @dev Revokes `role` from the calling account.
   *
   * Roles are often managed via {grantRole} and {revokeRole}: this function's
   * purpose is to provide a mechanism for accounts to lose their privileges
   * if they are compromised (such as when a trusted device is misplaced).
   *
   * If the calling account had been revoked `role`, emits a {RoleRevoked}
   * event.
   *
   * Requirements:
   *
   * - the caller must be `account`.
   *
   * May emit a {RoleRevoked} event.
   */
  function renounceRole(bytes32 role, address account) public virtual override {
    require(account == _msgSender(), "AccessControl: can only renounce roles for self");

    _revokeRole(role, account);
  }

  /**
   * @dev Grants `role` to `account`.
   *
   * If `account` had not been already granted `role`, emits a {RoleGranted}
   * event. Note that unlike {grantRole}, this function doesn't perform any
   * checks on the calling account.
   *
   * May emit a {RoleGranted} event.
   *
   * [WARNING]
   * ====
   * This function should only be called from the constructor when setting
   * up the initial roles for the system.
   *
   * Using this function in any other way is effectively circumventing the admin
   * system imposed by {AccessControl}.
   * ====
   *
   * NOTE: This function is deprecated in favor of {_grantRole}.
   */
  function _setupRole(bytes32 role, address account) internal virtual {
    _grantRole(role, account);
  }

  /**
   * @dev Sets `adminRole` as ``role``'s admin role.
   *
   * Emits a {RoleAdminChanged} event.
   */
  function _setRoleAdmin(bytes32 role, bytes32 adminRole) internal virtual {
    bytes32 previousAdminRole = getRoleAdmin(role);
    _roles[role].adminRole = adminRole;
    emit RoleAdminChanged(role, previousAdminRole, adminRole);
  }

  /**
   * @dev Grants `role` to `account`.
   *
   * Internal function without access restriction.
   *
   * May emit a {RoleGranted} event.
   */
  function _grantRole(bytes32 role, address account) internal virtual {
    if (!hasRole(role, account)) {
      _roles[role].members[account] = true;
      emit RoleGranted(role, account, _msgSender());
    }
  }

  /**
   * @dev Revokes `role` from `account`.
   *
   * Internal function without access restriction.
   *
   * May emit a {RoleRevoked} event.
   */
  function _revokeRole(bytes32 role, address account) internal virtual {
    if (hasRole(role, account)) {
      _roles[role].members[account] = false;
      emit RoleRevoked(role, account, _msgSender());
    }
  }

  /**
   * @dev This empty reserved space is put in place to allow future versions to add new
   * variables without shifting down storage in the inheritance chain.
   * See https://docs.openzeppelin.com/contracts/4.x/upgradeable#storage_gaps
   */
  uint256[49] private __gap;
}

// File @openzeppelin/contracts-upgradeable/security/ReentrancyGuardUpgradeable.sol@v4.9.3

// LICENSE-License-Identifier: MIT
// OpenZeppelin Contracts (last updated v4.9.0) (security/ReentrancyGuard.sol)

pragma solidity ^0.8.0;

/**
 * @dev Contract module that helps prevent reentrant calls to a function.
 *
 * Inheriting from `ReentrancyGuard` will make the {nonReentrant} modifier
 * available, which can be applied to functions to make sure there are no nested
 * (reentrant) calls to them.
 *
 * Note that because there is a single `nonReentrant` guard, functions marked as
 * `nonReentrant` may not call one another. This can be worked around by making
 * those functions `private`, and then adding `external` `nonReentrant` entry
 * points to them.
 *
 * TIP: If you would like to learn more about reentrancy and alternative ways
 * to protect against it, check out our blog post
 * https://blog.openzeppelin.com/reentrancy-after-istanbul/[Reentrancy After Istanbul].
 */
abstract contract ReentrancyGuardUpgradeable is Initializable {
  // Booleans are more expensive than uint256 or any type that takes up a full
  // word because each write operation emits an extra SLOAD to first read the
  // slot's contents, replace the bits taken up by the boolean, and then write
  // back. This is the compiler's defense against contract upgrades and
  // pointer aliasing, and it cannot be disabled.

  // The values being non-zero value makes deployment a bit more expensive,
  // but in exchange the refund on every call to nonReentrant will be lower in
  // amount. Since refunds are capped to a percentage of the total
  // transaction's gas, it is best to keep them low in cases like this one, to
  // increase the likelihood of the full refund coming into effect.
  uint256 private constant _NOT_ENTERED = 1;
  uint256 private constant _ENTERED = 2;

  uint256 private _status;

  function __ReentrancyGuard_init() internal onlyInitializing {
    __ReentrancyGuard_init_unchained();
  }

  function __ReentrancyGuard_init_unchained() internal onlyInitializing {
    _status = _NOT_ENTERED;
  }

  /**
   * @dev Prevents a contract from calling itself, directly or indirectly.
   * Calling a `nonReentrant` function from another `nonReentrant`
   * function is not supported. It is possible to prevent this from happening
   * by making the `nonReentrant` function external, and making it call a
   * `private` function that does the actual work.
   */
  modifier nonReentrant() {
    _nonReentrantBefore();
    _;
    _nonReentrantAfter();
  }

  function _nonReentrantBefore() private {
    // On the first call to nonReentrant, _status will be _NOT_ENTERED
    require(_status != _ENTERED, "ReentrancyGuard: reentrant call");

    // Any calls to nonReentrant after this point will fail
    _status = _ENTERED;
  }

  function _nonReentrantAfter() private {
    // By storing the original value once again, a refund is triggered (see
    // https://eips.ethereum.org/EIPS/eip-2200)
    _status = _NOT_ENTERED;
  }

  /**
   * @dev Returns true if the reentrancy guard is currently set to "entered", which indicates there is a
   * `nonReentrant` function in the call stack.
   */
  function _reentrancyGuardEntered() internal view returns (bool) {
    return _status == _ENTERED;
  }

  /**
   * @dev This empty reserved space is put in place to allow future versions to add new
   * variables without shifting down storage in the inheritance chain.
   * See https://docs.openzeppelin.com/contracts/4.x/upgradeable#storage_gaps
   */
  uint256[49] private __gap;
}

// File contracts/interfaces/IL2MessageManager.sol

// LICENSE-License-Identifier: Apache-2.0
pragma solidity 0.8.19;

interface IL2MessageManager {
  /**
   * @dev Emitted when L1->L2 message hashes have been added to L2 storage.
   */
  event L1L2MessageHashesAddedToInbox(bytes32[] messageHashes);

  /**
   * @dev Thrown when the message hashes list length is higher than one hundred.
   */
  error MessageHashesListLengthHigherThanOneHundred(uint256 length);

  /**
   * @dev Thrown when the message hashes array is empty.
   */
  error EmptyMessageHashesArray();

  /**
   * @dev Thrown when the message does not exist or has already been claimed.
   */
  error MessageDoesNotExistOrHasAlreadyBeenClaimed(bytes32 messageHash);

  /**
   * @notice Anchor L1-> L2 message hashes.
   * @param _messageHashes New message hashes to anchor on L2.
   */
  function addL1L2MessageHashes(bytes32[] calldata _messageHashes) external;
}

// File contracts/interfaces/IPauseManager.sol

// LICENSE-License-Identifier: Apache-2.0
pragma solidity 0.8.19;

interface IPauseManager {
  /**
   * @dev Thrown when a specific pause type is paused.
   */
  error IsPaused(bytes32 pauseType);

  /**
   * @dev Thrown when a specific pause type is not paused and expected to be.
   */
  error IsNotPaused(bytes32 pauseType);

  /**
   * @dev Emitted when a pause type is paused.
   */
  event Paused(address messageSender, bytes32 indexed pauseType);

  /**
   * @dev Emitted when a pause type is unpaused.
   */
  event UnPaused(address messageSender, bytes32 indexed pauseType);
}

// File contracts/messageService/lib/PauseManager.sol

// LICENSE-License-Identifier: AGPL-3.0

pragma solidity 0.8.19;

/**
 * @title Contract to manage cross-chain function pausing.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract PauseManager is Initializable, IPauseManager, AccessControlUpgradeable {
  bytes32 public constant PAUSE_MANAGER_ROLE = keccak256("PAUSE_MANAGER_ROLE");

  bytes32 public constant GENERAL_PAUSE_TYPE = keccak256("GENERAL_PAUSE_TYPE");
  bytes32 public constant L1_L2_PAUSE_TYPE = keccak256("L1_L2_PAUSE_TYPE");
  bytes32 public constant L2_L1_PAUSE_TYPE = keccak256("L2_L1_PAUSE_TYPE");
  bytes32 public constant PROVING_SYSTEM_PAUSE_TYPE = keccak256("PROVING_SYSTEM_PAUSE_TYPE");

  mapping(bytes32 => bool) public pauseTypeStatuses;

  uint256[10] private __gap;

  /**
   * @dev Modifier to make a function callable only when the type is not paused.
   *
   * Requirements:
   *
   * - The type must not be paused.
   */
  modifier whenTypeNotPaused(bytes32 _pauseType) {
    _requireTypeNotPaused(_pauseType);
    _;
  }

  /**
   * @dev Modifier to make a function callable only when the type is paused.
   *
   * Requirements:
   *
   * - The type must not be paused.
   */
  modifier whenTypePaused(bytes32 _pauseType) {
    _requireTypePaused(_pauseType);
    _;
  }

  /**
   * @dev Throws if the type is not paused.
   * @param _pauseType The keccak256 pause type being checked.
   */
  function _requireTypePaused(bytes32 _pauseType) internal view virtual {
    if (!pauseTypeStatuses[_pauseType]) {
      revert IsNotPaused(_pauseType);
    }
  }

  /**
   * @dev Throws if the type is paused.
   * @param _pauseType The keccak256 pause type being checked.
   */
  function _requireTypeNotPaused(bytes32 _pauseType) internal view virtual {
    if (pauseTypeStatuses[_pauseType]) {
      revert IsPaused(_pauseType);
    }
  }

  /**
   * @notice Pauses functionality by specific type.
   * @dev Requires PAUSE_MANAGER_ROLE.
   * @param _pauseType keccak256 pause type.
   */
  function pauseByType(bytes32 _pauseType) external whenTypeNotPaused(_pauseType) onlyRole(PAUSE_MANAGER_ROLE) {
    pauseTypeStatuses[_pauseType] = true;
    emit Paused(_msgSender(), _pauseType);
  }

  /**
   * @notice Unpauses functionality by specific type.
   * @dev Requires PAUSE_MANAGER_ROLE.
   * @param _pauseType keccak256 pause type.
   */
  function unPauseByType(bytes32 _pauseType) external whenTypePaused(_pauseType) onlyRole(PAUSE_MANAGER_ROLE) {
    pauseTypeStatuses[_pauseType] = false;
    emit UnPaused(_msgSender(), _pauseType);
  }
}

// File contracts/messageService/l2/L2MessageManager.sol

// LICENSE-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

/**
 * @title Contract to manage cross-chain message hashes storage and statuses on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L2MessageManager is Initializable, PauseManager, IL2MessageManager {
  uint8 public constant INBOX_STATUS_UNKNOWN = 0;
  uint8 public constant INBOX_STATUS_RECEIVED = 1;
  uint8 public constant INBOX_STATUS_CLAIMED = 2;

  bytes32 public constant L1_L2_MESSAGE_SETTER_ROLE = keccak256("L1_L2_MESSAGE_SETTER_ROLE");

  /**
   * @dev Mapping to store L1->L2 message hashes status.
   * @dev messageHash => messageStatus (0: unknown, 1: received, 2: claimed).
   */
  mapping(bytes32 => uint256) public inboxL1L2MessageStatus;

  /// @dev Keep free storage slots for future implementation updates to avoid storage collision.
  // *******************************************************************************************
  // NB: THIS GAP HAS BEEN PUSHED OUT IN FAVOUR OF THE GAP INSIDE THE REENTRANCY CODE
  //uint256[50] private __gap;
  // NB: DO NOT USE THIS GAP
  // *******************************************************************************************

  /**
   * @notice Initialises L2 message manager contract.
   * @param _l1l2MessageSetter The address owning the L1_L2_MESSAGE_SETTER_ROLE role.
   */
  function __L2MessageManager_init(address _l1l2MessageSetter) internal onlyInitializing {
    _grantRole(L1_L2_MESSAGE_SETTER_ROLE, _l1l2MessageSetter);
  }

  /**
   * @notice Add a cross-chain L1->L2 message hashes in storage.
   * @dev Only address that has the role 'L1_L2_MESSAGE_SETTER_ROLE' are allowed to call this function.
   * @param _messageHashes Message hashes array.
   */
  function addL1L2MessageHashes(bytes32[] calldata _messageHashes) external onlyRole(L1_L2_MESSAGE_SETTER_ROLE) {
    uint256 messageHashesLength = _messageHashes.length;

    if (messageHashesLength > 100) {
      revert MessageHashesListLengthHigherThanOneHundred(messageHashesLength);
    }

    for (uint256 i; i < messageHashesLength; ) {
      bytes32 messageHash = _messageHashes[i];
      if (inboxL1L2MessageStatus[messageHash] == INBOX_STATUS_UNKNOWN) {
        inboxL1L2MessageStatus[messageHash] = INBOX_STATUS_RECEIVED;
      }
      unchecked {
        i++;
      }
    }

    emit L1L2MessageHashesAddedToInbox(_messageHashes);
  }

  /**
   * @notice Update the status of L1->L2 message when a user claims a message on L2.
   * @param _messageHash Hash of the message.
   */
  function _updateL1L2MessageStatusToClaimed(bytes32 _messageHash) internal {
    if (inboxL1L2MessageStatus[_messageHash] != INBOX_STATUS_RECEIVED) {
      revert MessageDoesNotExistOrHasAlreadyBeenClaimed(_messageHash);
    }

    inboxL1L2MessageStatus[_messageHash] = INBOX_STATUS_CLAIMED;
  }
}

// File contracts/interfaces/IGenericErrors.sol

// LICENSE-License-Identifier: Apache-2.0
pragma solidity 0.8.19;

interface IGenericErrors {
  /**
   * @dev Thrown when a parameter is the zero address.
   */
  error ZeroAddressNotAllowed();
}

// File contracts/interfaces/IMessageService.sol

// LICENSE-License-Identifier: Apache-2.0
pragma solidity 0.8.19;

interface IMessageService {
  /**
   * @dev Emitted when a message is sent.
   * @dev We include the message hash to save hashing costs on the rollup.
   */
  event MessageSent(
    address indexed _from,
    address indexed _to,
    uint256 _fee,
    uint256 _value,
    uint256 _nonce,
    bytes _calldata,
    bytes32 indexed _messageHash
  );

  /**
   * @dev Emitted when a message is claimed.
   */
  event MessageClaimed(bytes32 indexed _messageHash);

  /**
   * @dev Thrown when fees are lower than the minimum fee.
   */
  error FeeTooLow();

  /**
   * @dev Thrown when fees are lower than value.
   */
  error ValueShouldBeGreaterThanFee();

  /**
   * @dev Thrown when the value sent is less than the fee.
   * @dev Value to forward on is msg.value - _fee.
   */
  error ValueSentTooLow();

  /**
   * @dev Thrown when the destination address reverts.
   */
  error MessageSendingFailed(address destination);

  /**
   * @dev Thrown when the recipient address reverts.
   */
  error FeePaymentFailed(address recipient);

  /**
   * @notice Sends a message for transporting from the given chain.
   * @dev This function should be called with a msg.value = _value + _fee. The fee will be paid on the destination chain.
   * @param _to The destination address on the destination chain.
   * @param _fee The message service fee on the origin chain.
   * @param _calldata The calldata used by the destination message service to call the destination contract.
   */
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable;

  /**
   * @notice Deliver a message to the destination chain.
   * @notice Is called automatically by the Postman, dApp or end user.
   * @param _from The msg.sender calling the origin message service.
   * @param _to The destination address on the destination chain.
   * @param _value The value to be transferred to the destination address.
   * @param _fee The message service fee on the origin chain.
   * @param _feeRecipient Address that will receive the fees.
   * @param _calldata The calldata used by the destination message service to call/forward to the destination contract.
   * @param _nonce Unique message number.
   */
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) external;

  /**
   * @notice Returns the original sender of the message on the origin layer.
   * @return The original sender of the message on the origin layer.
   */
  function sender() external view returns (address);
}

// File contracts/interfaces/IRateLimiter.sol

// LICENSE-License-Identifier: Apache-2.0
pragma solidity 0.8.19;

interface IRateLimiter {
  /**
   * @dev Thrown when an amount breaches the limit in the period.
   */
  error RateLimitExceeded();

  /**
   * @dev Thrown when the period is initialised to zero.
   */
  error PeriodIsZero();

  /**
   * @dev Thrown when the limit is initialised to zero.
   */
  error LimitIsZero();

  /**
   * @dev Emitted when the amount in the period is reset to zero.
   */
  event AmountUsedInPeriodReset(address indexed resettingAddress);

  /**
   * @dev Emitted when the limit is changed.
   * @dev If the current used amount is higher than the new limit, the used amount is lowered to the limit.
   */
  event LimitAmountChanged(
    address indexed amountChangeBy,
    uint256 amount,
    bool amountUsedLoweredToLimit,
    bool usedAmountResetToZero
  );

  /**
   * @notice Resets the rate limit amount to the amount specified.
   * @param _amount sets the new limit amount.
   */
  function resetRateLimitAmount(uint256 _amount) external;

  /**
   * @notice Resets the amount used in the period to zero.
   */
  function resetAmountUsedInPeriod() external;
}

// File contracts/messageService/lib/RateLimiter.sol

// LICENSE-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

/**
 * @title Rate Limiter by period and amount using the block timestamp.
 * @author ConsenSys Software Inc.
 * @notice You can use this control numeric limits over a period using timestamp.
 * @custom:security-contact security-report@linea.build
 */
contract RateLimiter is Initializable, IRateLimiter, AccessControlUpgradeable {
  bytes32 public constant RATE_LIMIT_SETTER_ROLE = keccak256("RATE_LIMIT_SETTER_ROLE");

  uint256 public periodInSeconds; // how much time before limit resets.
  uint256 public limitInWei; // max ether to withdraw per period.

  // @dev Public for ease of consumption.
  // @notice The time at which the current period ends at.
  uint256 public currentPeriodEnd;

  // @dev Public for ease of consumption.
  // @notice Amounts already withdrawn this period.
  uint256 public currentPeriodAmountInWei;

  uint256[10] private __gap;

  /**
   * @notice Initialises the limits and period for the rate limiter.
   * @param _periodInSeconds The length of the period in seconds.
   * @param _limitInWei The limit allowed in the period in Wei.
   */
  function __RateLimiter_init(uint256 _periodInSeconds, uint256 _limitInWei) internal onlyInitializing {
    if (_periodInSeconds == 0) {
      revert PeriodIsZero();
    }

    if (_limitInWei == 0) {
      revert LimitIsZero();
    }

    periodInSeconds = _periodInSeconds;
    limitInWei = _limitInWei;
    currentPeriodEnd = block.timestamp + _periodInSeconds;
  }

  /**
   * @notice Increments the amount used in the period.
   * @dev The amount determining logic is external to this (e.g. fees are included when calling here).
   * @dev Reverts if the limit is breached.
   * @param _usedAmount The amount used to be added.
   */
  function _addUsedAmount(uint256 _usedAmount) internal {
    uint256 currentPeriodAmountTemp;

    if (currentPeriodEnd < block.timestamp) {
      currentPeriodEnd = block.timestamp + periodInSeconds;
      currentPeriodAmountTemp = _usedAmount;
    } else {
      currentPeriodAmountTemp = currentPeriodAmountInWei + _usedAmount;
    }

    if (currentPeriodAmountTemp > limitInWei) {
      revert RateLimitExceeded();
    }

    currentPeriodAmountInWei = currentPeriodAmountTemp;
  }

  /**
   * @notice Resets the rate limit amount.
   * @dev If the used amount is higher, it is set to the limit to avoid confusion/issues.
   * @dev Only the RATE_LIMIT_SETTER_ROLE is allowed to execute this function.
   * @dev Emits the LimitAmountChanged event.
   * @dev usedLimitAmountToSet will use the default value of zero if period has expired
   * @param _amount The amount to reset the limit to.
   */
  function resetRateLimitAmount(uint256 _amount) external onlyRole(RATE_LIMIT_SETTER_ROLE) {
    uint256 usedLimitAmountToSet;
    bool amountUsedLoweredToLimit;
    bool usedAmountResetToZero;

    if (currentPeriodEnd < block.timestamp) {
      currentPeriodEnd = block.timestamp + periodInSeconds;
      usedAmountResetToZero = true;
    } else {
      if (_amount < currentPeriodAmountInWei) {
        usedLimitAmountToSet = _amount;
        amountUsedLoweredToLimit = true;
      }
    }

    limitInWei = _amount;

    if (usedAmountResetToZero || amountUsedLoweredToLimit) {
      currentPeriodAmountInWei = usedLimitAmountToSet;
    }

    emit LimitAmountChanged(_msgSender(), _amount, amountUsedLoweredToLimit, usedAmountResetToZero);
  }

  /**
   * @notice Resets the amount used to zero.
   * @dev Only the RATE_LIMIT_SETTER_ROLE is allowed to execute this function.
   * @dev Emits the AmountUsedInPeriodReset event.
   */
  function resetAmountUsedInPeriod() external onlyRole(RATE_LIMIT_SETTER_ROLE) {
    currentPeriodAmountInWei = 0;

    emit AmountUsedInPeriodReset(_msgSender());
  }
}

// File contracts/messageService/l2/L2MessageService.sol

// LICENSE-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

/**
 * @title Contract to manage cross-chain messaging on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract L2MessageServiceLineaMainnet is
  Initializable,
  RateLimiter,
  L2MessageManager,
  ReentrancyGuardUpgradeable,
  IMessageService,
  IGenericErrors
{
  // Keep free storage slots for future implementation updates to avoid storage collision.
  // @dev NB: Take note that this is at the beginning of the file where other storage gaps,
  // are at the end of files. Be careful with how storage is adjusted on upgrades.
  uint256[50] private __gap_L2MessageService;

  bytes32 public constant MINIMUM_FEE_SETTER_ROLE = keccak256("MINIMUM_FEE_SETTER_ROLE");

  address private _messageSender;

  // @dev initialise to save user cost with existing slot.
  uint256 public nextMessageNumber;

  // @dev initialise minimumFeeInWei variable.
  uint256 public minimumFeeInWei;

  // @dev adding these should not affect storage as they are constants and are stored in bytecode.
  uint256 private constant REFUND_OVERHEAD_IN_GAS = 47500;

  address private constant DEFAULT_SENDER_ADDRESS = address(123456789);

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initialises underlying message service dependencies.
   * @param _securityCouncil The address owning the security council role.
   * @param _l1l2MessageSetter The address owning the add L1L2MessageHashes functionality.
   * @param _rateLimitPeriod The period to rate limit against.
   * @param _rateLimitAmount The limit allowed for withdrawing the period.
   */
  function initialize(
    address _securityCouncil,
    address _l1l2MessageSetter,
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount
  ) public initializer {
    if (_securityCouncil == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_l1l2MessageSetter == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    __ERC165_init();
    __Context_init();
    __AccessControl_init();
    __RateLimiter_init(_rateLimitPeriod, _rateLimitAmount);
    __L2MessageManager_init(_l1l2MessageSetter);

    nextMessageNumber = 1;

    _grantRole(DEFAULT_ADMIN_ROLE, _securityCouncil);
    _grantRole(MINIMUM_FEE_SETTER_ROLE, _securityCouncil);
    _grantRole(RATE_LIMIT_SETTER_ROLE, _securityCouncil);
    _grantRole(PAUSE_MANAGER_ROLE, _securityCouncil);

    _messageSender = DEFAULT_SENDER_ADDRESS;
  }

  /**
   * @notice Adds a message for sending cross-chain and emits a relevant event.
   * @dev The message number is preset and only incremented at the end if successful for the next caller.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _calldata The calldata to pass to the recipient.
   */
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {
    _requireTypeNotPaused(L2_L1_PAUSE_TYPE);
    _requireTypeNotPaused(GENERAL_PAUSE_TYPE);

    if (_to == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_fee > msg.value) {
      revert ValueSentTooLow();
    }

    uint256 coinbaseFee = minimumFeeInWei;

    if (_fee < coinbaseFee) {
      revert FeeTooLow();
    }

    uint256 postmanFee;
    uint256 valueSent;

    unchecked {
      postmanFee = _fee - coinbaseFee;
      valueSent = msg.value - _fee;
    }

    uint256 messageNumber = nextMessageNumber;
    /// @dev Rate limit and revert is in the rate limiter.
    _addUsedAmount(valueSent + postmanFee);

    bytes32 messageHash = keccak256(abi.encode(msg.sender, _to, postmanFee, valueSent, messageNumber, _calldata));

    nextMessageNumber++;

    (bool success, ) = block.coinbase.call{ value: coinbaseFee }("");
    if (!success) {
      revert FeePaymentFailed(block.coinbase);
    }

    emit MessageSent(msg.sender, _to, postmanFee, valueSent, messageNumber, _calldata, messageHash);
  }

  /**
   * @notice Claims and delivers a cross-chain message.
   * @dev _feeRecipient Can be set to address(0) to receive as msg.sender.
   * @dev messageSender Is set temporarily when claiming and reset post.
   * @param _from The address of the original sender.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _value The value to be transferred to the destination address.
   * @param _feeRecipient The recipient for the fee.
   * @param _calldata The calldata to pass to the recipient.
   * @param _nonce The unique auto generated message number used when sending the message.
   */
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) external nonReentrant distributeFees(_fee, _to, _calldata, _feeRecipient) {
    _requireTypeNotPaused(L1_L2_PAUSE_TYPE);
    _requireTypeNotPaused(GENERAL_PAUSE_TYPE);

    bytes32 messageHash = keccak256(abi.encode(_from, _to, _fee, _value, _nonce, _calldata));

    /// @dev Status check and revert is in the message manager.
    _updateL1L2MessageStatusToClaimed(messageHash);

    _messageSender = _from;

    (bool callSuccess, bytes memory returnData) = _to.call{ value: _value }(_calldata);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(32, returnData), data_size)
        }
      } else {
        revert MessageSendingFailed(_to);
      }
    }

    _messageSender = DEFAULT_SENDER_ADDRESS;
    emit MessageClaimed(messageHash);
  }

  /**
   * @notice The Fee Manager sets a minimum fee to address DOS protection.
   * @param _feeInWei New minimum fee in Wei.
   */
  function setMinimumFee(uint256 _feeInWei) external onlyRole(MINIMUM_FEE_SETTER_ROLE) {
    minimumFeeInWei = _feeInWei;
  }

  /**
   * @dev The _messageSender address is set temporarily when claiming.
   * @return _messageSender address.
   */
  function sender() external view returns (address) {
    return _messageSender;
  }

  /**
   * @notice Function to receive funds for liquidity purposes.
   */
  receive() external payable virtual {}

  /**
   * @notice The unspent fee is refunded if applicable.
   * @param _feeInWei The fee paid for delivery in Wei.
   * @param _to The recipient of the message and gas refund.
   * @param _calldata The calldata of the message.
   */
  modifier distributeFees(
    uint256 _feeInWei,
    address _to,
    bytes calldata _calldata,
    address _feeRecipient
  ) {
    //pre-execution
    uint256 startingGas = gasleft();
    _;
    //post-execution

    // we have a fee
    if (_feeInWei > 0) {
      // default postman fee
      uint256 deliveryFee = _feeInWei;

      // do we have empty calldata?
      if (_calldata.length == 0) {
        bool isDestinationEOA;

        assembly {
          isDestinationEOA := iszero(extcodesize(_to))
        }

        // are we calling an EOA
        if (isDestinationEOA) {
          // initial + cost to call and refund minus gasleft
          deliveryFee = (startingGas + REFUND_OVERHEAD_IN_GAS - gasleft()) * tx.gasprice;

          if (_feeInWei > deliveryFee) {
            payable(_to).send(_feeInWei - deliveryFee);
          } else {
            deliveryFee = _feeInWei;
          }
        }
      }

      address feeReceiver = _feeRecipient == address(0) ? msg.sender : _feeRecipient;

      bool callSuccess = payable(feeReceiver).send(deliveryFee);
      if (!callSuccess) {
        revert FeePaymentFailed(feeReceiver);
      }
    }
  }
}
