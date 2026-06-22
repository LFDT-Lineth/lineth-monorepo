// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

contract TestContract {
  event TestEvent(address indexed sender, uint256 value);

  event TestEventWithoutIndexing(address sender, uint256 value);

  struct TestStruct {
    uint256 value;
  }

  TestStruct public testStruct;

  mapping(address => uint256) public storageMap;

  // KECCAK
  function testKeccak(string memory _input) external returns (bytes32) {
    return keccak256(abi.encodePacked(_input));
  }

  function testKeccak2(string memory _input) external returns (bytes32) {
    return keccak256(abi.encode(_input));
  }

  function testKeccak3(uint256 _count) external {
    for (uint256 i; i < _count; i++) {
      keccak256(abi.encodePacked(i));
    }
  }

  function testKeccak4(uint256 _count) external {
    for (uint256 i; i < _count; i++) {
      keccak256(abi.encodePacked(i));
    }
  }

  function testAddmod(uint256 _count, uint256 _mod) external {
    for (uint256 i; i < _count; i++) {
      addmod(i, i + 1, _mod);
    }
  }

  function testMulmod(uint256 _count, uint256 _mod) external {
    for (uint256 i; i < _count; i++) {
      mulmod(i, i + 1, _mod);
    }
  }

  // STORAGE
  function addToStorageMap(address _key, uint256 _value) public {
    storageMap[_key] = _value;
  }

  // STORAGE DELETE
  function deleteFromStorageMap(address _key) public {
    delete storageMap[_key];
  }

  // EVENTS
  function testIndexedEventEmitting(uint256 _value) public {
    emit TestEvent(msg.sender, _value);
  }

  function testEventEmitting(uint256 _value) public {
    emit TestEventWithoutIndexing(msg.sender, _value);
  }

  // External Call
  function testExternalCalls(address target, bytes memory _data) public {
    (bool success, ) = target.call(_data);
    require(success, "Call failed");

    (success, ) = address(this).delegatecall(_data);
    require(success, "Delegatecall failed");
  }

  function testEncoding(string memory _input)
  public

  returns (bytes memory)
  {
    return abi.encode(_input);
  }

  function testEncodingPacked(string memory _input)
  public

  returns (bytes memory)
  {
    return abi.encodePacked(_input);
  }

  // STRUCT CREATION
  function createStruct(uint256 _value) public {
    TestStruct memory newStruct = TestStruct(_value);

    testStruct = newStruct;
  }
}
