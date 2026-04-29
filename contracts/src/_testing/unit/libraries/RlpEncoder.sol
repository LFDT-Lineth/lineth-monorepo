// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.33;

/**
 * @title Library for RLP Encoding data.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 * @custom:attribution https://github.com/bakaoh/solidity-rlp-encode
 * @dev The internals have been significantly modified from the original for readability and gas.
 */
library RlpEncoder {
  /**
   * @notice Supporting data for encoding an EIP-2930/1559 access lists.
   * @dev contractAddress is the address where the storageKeys will be accessed.
   * @dev storageKeys contains the list of keys expected to be accessed at contractAddress.
   */
  struct AccessList {
    address contractAddress;
    bytes32[] storageKeys;
  }

  function _encodeBytes(bytes memory _bytesIn) internal pure returns (bytes memory encodedBytes) {
    if (_bytesIn.length == 1 && uint8(_bytesIn[0]) < 0x80) return _bytesIn;

    bytes memory lengthPrefix = _encodeLength(_bytesIn.length, 128);
    uint256 prefixLen = lengthPrefix.length;
    uint256 dataLen = _bytesIn.length;

    assembly {
      let totalLen := add(prefixLen, dataLen)
      encodedBytes := mload(0x40)
      mstore(encodedBytes, totalLen)

      let dest := add(encodedBytes, 0x20)
      let src := add(lengthPrefix, 0x20)
      mcopy(dest, src, prefixLen)

      dest := add(dest, prefixLen)
      src := add(_bytesIn, 0x20)
      mcopy(dest, src, dataLen)

      mstore(0x40, add(add(encodedBytes, 0x20), totalLen))
    }
  }

  function _encodeUint(uint256 _uintIn) internal pure returns (bytes memory encodedBytes) {
    encodedBytes = _encodeBytes(_toBinary(_uintIn));
  }

  /**
   * @notice Internal function that encodes an int value as bytes.
   * @param _intIn The int to encode.
   * @return encodedBytes The int encoded as bytes.
   */
  function _encodeInt(int256 _intIn) internal pure returns (bytes memory encodedBytes) {
    encodedBytes = _encodeUint(uint256(_intIn));
  }

  /**
   * @notice Internal function that encodes a address value as bytes.
   * @param _addressIn The address to be encoded.
   * @return encodedBytes The address encoded as bytes.
   */
  function _encodeAddress(address _addressIn) internal pure returns (bytes memory encodedBytes) {
    // RLP for 20 bytes: prefix 0x94 (0x80 + 20), then address bytes.
    encodedBytes = new bytes(21);
    encodedBytes[0] = 0x94;
    assembly {
      mstore(add(encodedBytes, 0x21), shl(96, _addressIn))
    }
  }

  /**
   * @notice Internal function that encodes a string value as bytes.
   * @param _stringIn The string to be encoded.
   * @return encodedBytes The string encoded as bytes.
   */
  function _encodeString(string memory _stringIn) internal pure returns (bytes memory encodedBytes) {
    encodedBytes = _encodeBytes(bytes(_stringIn));
  }

  /**
   * @notice Internal function that encodes a bool value as bytes.
   * @param _boolIn The bool to be encoded.
   * @return encodedBytes The bool encoded as bytes.
   */
  function _encodeBool(bool _boolIn) internal pure returns (bytes memory encodedBytes) {
    encodedBytes = new bytes(1);
    encodedBytes[0] = (_boolIn ? bytes1(0x01) : bytes1(0x80));
  }

  /**
   * @notice Internal function that flattens a bytes array and encodes it.
   * @param _bytesToEncode The bytes array to be encoded.
   * @return encodedBytes The bytes array encoded as bytes.
   */
  function _encodeList(bytes[] memory _bytesToEncode) internal pure returns (bytes memory encodedBytes) {
    encodedBytes = _encodeList(_bytesToEncode, _bytesToEncode.length);
  }

  /**
   * @notice Internal function that flattens a bytes array and encodes it.
   * @param _bytesToEncode The bytes array to be encoded.
   * @param _bytesListLengthToEncode The first n items in the list to encode.
   * @return encodedBytes The bytes array encoded as bytes.
   */
  function _encodeList(
    bytes[] memory _bytesToEncode,
    uint256 _bytesListLengthToEncode
  ) internal pure returns (bytes memory encodedBytes) {
    encodedBytes = _flatten(_bytesToEncode, _bytesListLengthToEncode);
    encodedBytes = abi.encodePacked(_encodeLength(encodedBytes.length, 192), encodedBytes);
  }

  function _encodeAccessList(AccessList[] memory _accesslist) internal pure returns (bytes memory encodedBytes) {
    uint256 listLength = _accesslist.length;
    bytes[] memory encodedAccessList = new bytes[](listLength);

    for (uint256 i; i < listLength; i++) {
      bytes32[] memory storageKeys = _accesslist[i].storageKeys;
      uint256 keyCount = storageKeys.length;

      bytes[] memory encodedKeys = new bytes[](keyCount);
      for (uint256 j; j < keyCount; j++) {
        encodedKeys[j] = _encodeBytes(abi.encodePacked(storageKeys[j]));
      }

      bytes[] memory accountTuple = new bytes[](2);
      accountTuple[0] = _encodeAddress(_accesslist[i].contractAddress);
      accountTuple[1] = _encodeList(encodedKeys);

      encodedAccessList[i] = _encodeList(accountTuple);
    }

    encodedBytes = _encodeList(encodedAccessList);
  }

  /**
   * @notice Private function that encodes an integer in big endian binary form with no leading zeroes.
   * @dev The zero length check is critical as the case is not considered in the rest of the checking.
   * @param _uintValue The uint value to be encoded.
   * @return encodedBytes The encoded uint.
   */
  function _toBinary(uint256 _uintValue) private pure returns (bytes memory encodedBytes) {
    if (_uintValue == 0) return new bytes(0);

    assembly {
      encodedBytes := mload(0x40)
      let sectionStart := 0
      let sectionLength := 128
      for {} gt(sectionLength, 7) {
        sectionLength := shr(1, sectionLength)
      } {
        if iszero(shr(sub(256, add(sectionLength, sectionStart)), _uintValue)) {
          sectionStart := add(sectionStart, sectionLength)
          continue
        }
      }

      let length := div(sub(256, sectionStart), 8)
      mstore(encodedBytes, length)

      let writePtr := add(encodedBytes, 0x20)

      for {
        let j := 0
      } lt(j, length) {
        j := add(j, 1)
      } {
        mstore8(add(writePtr, j), and(shr(mul(sub(length, add(j, 1)), 8), _uintValue), 0xff))
      }

      mstore(0x40, add(add(encodedBytes, 0x20), length))
    }
  }

  /**
   * @notice Private function that encodes length.
   * @param _itemLength The length of the item.
   * @param _offset The item's offset.
   * @return encodedBytes The bytes of the length encoded.
   */
  function _encodeLength(uint256 _itemLength, uint256 _offset) private pure returns (bytes memory encodedBytes) {
    assembly {
      encodedBytes := mload(0x40)

      switch lt(_itemLength, 56)
      case 1 {
        mstore8(add(encodedBytes, 0x20), add(_itemLength, _offset))
        mstore(encodedBytes, 1)
        mstore(0x40, add(encodedBytes, 0x21))
      }
      default {
        let temp := _itemLength
        let lengthOfLength := 0

        for {} gt(temp, 0) {} {
          lengthOfLength := add(lengthOfLength, 1)
          temp := shr(8, temp)
        }

        mstore8(add(encodedBytes, 0x20), add(add(lengthOfLength, _offset), 55))

        for {
          let i := 0
        } lt(i, lengthOfLength) {
          i := add(i, 1)
        } {
          mstore8(add(add(encodedBytes, 0x21), i), and(shr(mul(8, sub(lengthOfLength, add(i, 1))), _itemLength), 0xff))
        }

        let totalLen := add(lengthOfLength, 1)
        mstore(encodedBytes, totalLen)
        mstore(0x40, add(add(encodedBytes, 0x20), totalLen))
      }
    }
  }

  /**
   * @custom:attribution https://github.com/sammayo/solidity-rlp-encoder
   * @notice Private function that flattens a list of byte strings into one byte string.
   * @dev mcopy is used for the Cancun EVM fork. See original for other forks.
   * @param _bytesList List of byte strings to flatten.
   * @param _bytesListLengthToEncode The first n items in the list to encode.
   * @return flattenedBytes The flattened byte string.
   */
  function _flatten(
    bytes[] memory _bytesList,
    uint256 _bytesListLengthToEncode
  ) private pure returns (bytes memory flattenedBytes) {
    if (_bytesListLengthToEncode == 0) return new bytes(0);

    assembly {
      flattenedBytes := mload(0x40)
      let writePtr := add(flattenedBytes, 0x20)
      let totalLen := 0
      let ptrList := add(_bytesList, 0x20)
      let endPtrList := add(ptrList, shl(5, _bytesListLengthToEncode))

      for {} lt(ptrList, endPtrList) {
        ptrList := add(ptrList, 0x20)
      } {
        let chunk := mload(ptrList)
        let len := mload(chunk)

        mcopy(writePtr, add(chunk, 0x20), len)
        writePtr := add(writePtr, len)
        totalLen := add(totalLen, len)
      }

      mstore(flattenedBytes, totalLen)
      mstore(0x40, writePtr)
    }
  }
}
