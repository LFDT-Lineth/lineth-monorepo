// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { ErrorAndDestructionTesting } from "./ErrorAndDestructionTesting.sol";

contract OpcodeTester {
  mapping(bytes2 => uint256) public opcodeExecutions;
  address public yulContract;
  bytes32 public rollingBlockDetailComputations;

  address transient contractBeingCalled;

  // The opcodes are logged here for completeness sake even though not used.
  // NOTE: For looping we make it 2 bytes instead of one, so the real value is actually missing the 00 from 0x0001 (0x01) etc.

  // 0x00 - 0x0F
  bytes2 private constant STOP = 0x0000;
  bytes2 private constant ADD = 0x0001;
  bytes2 private constant MUL = 0x0002;
  bytes2 private constant SUB = 0x0003;
  bytes2 private constant DIV = 0x0004;
  bytes2 private constant SDIV = 0x0005;
  bytes2 private constant MOD = 0x0006;
  bytes2 private constant SMOD = 0x0007;
  bytes2 private constant ADDMOD = 0x0008;
  bytes2 private constant MULMOD = 0x0009;
  bytes2 private constant EXP = 0x000A;
  bytes2 private constant SIGNEXTEND = 0x000B;

  // 0x10 - 0x1F
  bytes2 private constant LT = 0x0010;
  bytes2 private constant GT = 0x0011;
  bytes2 private constant SLT = 0x0012;
  bytes2 private constant SGT = 0x0013;
  bytes2 private constant EQ = 0x0014;
  bytes2 private constant ISZERO = 0x0015;
  bytes2 private constant AND = 0x0016;
  bytes2 private constant OR = 0x0017;
  bytes2 private constant XOR = 0x0018;
  bytes2 private constant NOT = 0x0019;
  bytes2 private constant BYTE = 0x001A;
  bytes2 private constant SHL = 0x001B;
  bytes2 private constant SHR = 0x001C;
  bytes2 private constant SAR = 0x001D;
  bytes2 private constant CLZ = 0x001E;

  // 0x20 - 0x2F
  bytes2 private constant KECCAK256 = 0x0020;

  // 0x30 - 0x3F
  bytes2 private constant ADDRESS = 0x0030;
  bytes2 private constant BALANCE = 0x0031;
  bytes2 private constant ORIGIN = 0x0032;
  bytes2 private constant CALLER = 0x0033;
  bytes2 private constant CALLVALUE = 0x0034;
  bytes2 private constant CALLDATALOAD = 0x0035;
  bytes2 private constant CALLDATASIZE = 0x0036;
  bytes2 private constant CALLDATACOPY = 0x0037;
  bytes2 private constant CODESIZE = 0x0038;
  bytes2 private constant CODECOPY = 0x0039;
  bytes2 private constant GASPRICE = 0x003A;
  bytes2 private constant EXTCODESIZE = 0x003B;
  bytes2 private constant EXTCODECOPY = 0x003C;
  bytes2 private constant RETURNDATASIZE = 0x003D;
  bytes2 private constant RETURNDATACOPY = 0x003E;
  bytes2 private constant EXTCODEHASH = 0x003F;

  // 0x40 - 0x4F
  bytes2 private constant BLOCKHASH = 0x0040;
  bytes2 private constant COINBASE = 0x0041;
  bytes2 private constant TIMESTAMP = 0x0042;
  bytes2 private constant NUMBER = 0x0043;
  bytes2 private constant DIFFICULTY = 0x0044;
  bytes2 private constant GASLIMIT = 0x0045;
  bytes2 private constant CHAINID = 0x0046;
  bytes2 private constant SELFBALANCE = 0x0047;
  bytes2 private constant BASEFEE = 0x0048;
  bytes2 private constant BLOBHASH = 0x0049;
  bytes2 private constant BLOBBASEFEE = 0x004a;

  // 0x50 - 0x5F
  bytes2 private constant POP = 0x0050;
  bytes2 private constant MLOAD = 0x0051;
  bytes2 private constant MSTORE = 0x0052;
  bytes2 private constant MSTORE8 = 0x0053;
  bytes2 private constant SLOAD = 0x0054;
  bytes2 private constant SSTORE = 0x0055;
  bytes2 private constant JUMP = 0x0056;
  bytes2 private constant JUMPI = 0x0057;
  bytes2 private constant PC = 0x0058;
  bytes2 private constant MSIZE = 0x0059;
  bytes2 private constant GAS = 0x005A;
  bytes2 private constant JUMPDEST = 0x005B;
  bytes2 private constant TLOAD = 0x005C;
  bytes2 private constant TSTORE = 0x005D;
  bytes2 private constant MCOPY = 0x005E;
  bytes2 private constant PUSH0 = 0x005F;

  // 0x60 - 0x7F
  bytes2 private constant PUSH1 = 0x0060;
  bytes2 private constant PUSH2 = 0x0061;
  bytes2 private constant PUSH3 = 0x0062;
  bytes2 private constant PUSH4 = 0x0063;
  bytes2 private constant PUSH5 = 0x0064;
  bytes2 private constant PUSH6 = 0x0065;
  bytes2 private constant PUSH7 = 0x0066;
  bytes2 private constant PUSH8 = 0x0067;
  bytes2 private constant PUSH9 = 0x0068;
  bytes2 private constant PUSH10 = 0x0069;
  bytes2 private constant PUSH11 = 0x006A;
  bytes2 private constant PUSH12 = 0x006B;
  bytes2 private constant PUSH13 = 0x006C;
  bytes2 private constant PUSH14 = 0x006D;
  bytes2 private constant PUSH15 = 0x006E;
  bytes2 private constant PUSH16 = 0x006F;
  bytes2 private constant PUSH17 = 0x0070;
  bytes2 private constant PUSH18 = 0x0071;
  bytes2 private constant PUSH19 = 0x0072;
  bytes2 private constant PUSH20 = 0x0073;
  bytes2 private constant PUSH21 = 0x0074;
  bytes2 private constant PUSH22 = 0x0075;
  bytes2 private constant PUSH23 = 0x0076;
  bytes2 private constant PUSH24 = 0x0077;
  bytes2 private constant PUSH25 = 0x0078;
  bytes2 private constant PUSH26 = 0x0079;
  bytes2 private constant PUSH27 = 0x007A;
  bytes2 private constant PUSH28 = 0x007B;
  bytes2 private constant PUSH29 = 0x007C;
  bytes2 private constant PUSH30 = 0x007D;
  bytes2 private constant PUSH31 = 0x007E;
  bytes2 private constant PUSH32 = 0x007F;

  // 0x80 - 0x8F
  bytes2 private constant DUP1 = 0x0080;
  bytes2 private constant DUP2 = 0x0081;
  bytes2 private constant DUP3 = 0x0082;
  bytes2 private constant DUP4 = 0x0083;
  bytes2 private constant DUP5 = 0x0084;
  bytes2 private constant DUP6 = 0x0085;
  bytes2 private constant DUP7 = 0x0086;
  bytes2 private constant DUP8 = 0x0087;
  bytes2 private constant DUP9 = 0x0088;
  bytes2 private constant DUP10 = 0x0089;
  bytes2 private constant DUP11 = 0x008A;
  bytes2 private constant DUP12 = 0x008B;
  bytes2 private constant DUP13 = 0x008C;
  bytes2 private constant DUP14 = 0x008D;
  bytes2 private constant DUP15 = 0x008E;
  bytes2 private constant DUP16 = 0x008F;

  // 0x90 - 0x9F
  bytes2 private constant SWAP1 = 0x0090;
  bytes2 private constant SWAP2 = 0x0091;
  bytes2 private constant SWAP3 = 0x0092;
  bytes2 private constant SWAP4 = 0x0093;
  bytes2 private constant SWAP5 = 0x0094;
  bytes2 private constant SWAP6 = 0x0095;
  bytes2 private constant SWAP7 = 0x0096;
  bytes2 private constant SWAP8 = 0x0097;
  bytes2 private constant SWAP9 = 0x0098;
  bytes2 private constant SWAP10 = 0x0099;
  bytes2 private constant SWAP11 = 0x009A;
  bytes2 private constant SWAP12 = 0x009B;
  bytes2 private constant SWAP13 = 0x009C;
  bytes2 private constant SWAP14 = 0x009D;
  bytes2 private constant SWAP15 = 0x009E;
  bytes2 private constant SWAP16 = 0x009F;

  // 0xA0 - 0xA4
  bytes2 private constant LOG0 = 0x00A0;
  bytes2 private constant LOG1 = 0x00A1;
  bytes2 private constant LOG2 = 0x00A2;
  bytes2 private constant LOG3 = 0x00A3;
  bytes2 private constant LOG4 = 0x00A4;

  // 0xF0 - 0xFF
  bytes2 private constant CREATE = 0x00F0;
  bytes2 private constant CALL = 0x00F1;
  bytes2 private constant CALLCODE = 0x00F2;
  bytes2 private constant RETURN = 0x00F3;
  bytes2 private constant DELEGATECALL = 0x00F4;
  bytes2 private constant CREATE2 = 0x00F5;
  bytes2 private constant STATICCALL = 0x00FA;
  bytes2 private constant REVERT = 0x00FD;
  bytes2 private constant INVALID = 0x00FE;
  bytes2 private constant SELFDESTRUCT = 0x00FF;

  constructor(address _yulContract) {
    yulContract = _yulContract;
  }

  function executeAllOpcodes() public payable {
    executeExternalCalls();

    incrementOpcodeExecutions();

    performMemoryCopying();

    storeRollingGlobalVariablesToState();

    executeClz();
  }

  function performMemoryCopying() private pure {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, 0x20)
      mstore(add(mPtr, 0x20), 0x01)
      mstore(add(mPtr, 0x40), 0x02)
      mstore(add(mPtr, 0x60), 0x03)
      mstore(add(mPtr, 0x80), 0x04)
      mcopy(add(mPtr, 0xA0), mPtr, 0xA0)
    }
  }

  function executeExternalCalls() private {
    ErrorAndDestructionTesting errorAndDestructingContract = new ErrorAndDestructionTesting();

    // TLOAD + TSTORE
    if (contractBeingCalled == address(0)) {
      contractBeingCalled = address(errorAndDestructingContract);
    }

    bool success;
    (success, ) = address(errorAndDestructingContract).call(abi.encodeWithSignature("externalRevert()"));

    // it should fail
    if (success) {
      revert("Error: externalRevert did not revert");
    }

    contractBeingCalled = address(0);

    (success, ) = address(errorAndDestructingContract).staticcall(abi.encodeWithSignature("externalRevert()"));

    // it should fail
    if (success) {
      revert("Error: externalRevert did not revert");
    }

    (success, ) = address(errorAndDestructingContract).call(abi.encodeWithSignature("callmeToSelfDestruct()"));
    // it should succeed
    if (!success) {
      revert("Error: revertcallmeToSelfDestruct Failed");
    }

    (success, ) = yulContract.call(abi.encodeWithSignature("executeAll()"));
    if (!success) {
      revert("executeAll on yulContract Failed");
    }
  }

  function incrementOpcodeExecutions() private {
    // 0x0000 - 0x000B
    for (uint16 i = 0x0000; i <= 0x000B; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x0010 - 0x001E
    for (uint16 i = 0x0010; i <= 0x001E; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x0020 - 0x000
    opcodeExecutions[KECCAK256] = opcodeExecutions[KECCAK256] + 1;

    // 0x0030 - 0x004A
    for (uint16 i = 0x0030; i <= 0x004A; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x0050 - 0x005F
    for (uint16 i = 0x0050; i <= 0x005F; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x0060 - 0x009F
    for (uint16 i = 0x0060; i <= 0x009F; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x00A0 - 0x00A4
    for (uint16 i = 0x00A0; i <= 0x00A4; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x00F0 - 0x00F5
    for (uint16 i = 0x00F0; i <= 0x00F5; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }

    // 0x00FA
    opcodeExecutions[STATICCALL] = opcodeExecutions[STATICCALL] + 1;

    // 0x00FD - 0x00FF
    for (uint16 i = 0x00FD; i <= 0x00FF; i += 0x0001) {
      opcodeExecutions[bytes2(i)] = opcodeExecutions[bytes2(i)] + 1;
    }
  }

  function storeRollingGlobalVariablesToState() private {
    bytes memory fieldsToHashSection1 = abi.encode(
      rollingBlockDetailComputations,
      parentBlocksHashes(),
      block.basefee,
      block.chainid,
      block.coinbase,
      block.prevrandao
    );

    bytes memory fieldsToHashSection2 = abi.encode(block.gaslimit, block.number, block.timestamp, gasleft());

    bytes memory fieldsToHashSection3 = abi.encode(blobhash(0), block.blobbasefee);

    bytes memory fieldsToHashSection4 = abi.encode(msg.data, msg.sender, msg.sig, msg.value, tx.gasprice, tx.origin);

    rollingBlockDetailComputations = keccak256(
      bytes.concat(
        bytes.concat(bytes.concat(fieldsToHashSection1, fieldsToHashSection2), fieldsToHashSection3),
        fieldsToHashSection4
      )
    );
  }

  function parentBlocksHashes() private view returns (bytes32[] memory) {
    uint256 endLookBack = 256;
    if (block.number < 256) {
      endLookBack = block.number;
    }
    bytes32[] memory blocksHashes = new bytes32[](endLookBack + 1);

    for (uint256 i = 0; i <= endLookBack; i++) {
      blocksHashes[i] = blockhash(block.number - i);
    }
    return blocksHashes;
  }

  function executeClz() public pure {
    assembly {
      // CLZ(0) = 256
      let r := clz(0)
      if iszero(eq(r, 256)) {
        revert(0, 0)
      }

      // CLZ(1) = 255
      r := clz(1)
      if iszero(eq(r, 255)) {
        revert(0, 0)
      }

      // CLZ(2) = 254
      r := clz(2)
      if iszero(eq(r, 254)) {
        revert(0, 0)
      }

      // CLZ(0xFF) = 248
      r := clz(0xFF)
      if iszero(eq(r, 248)) {
        revert(0, 0)
      }

      // CLZ(0x100) = 247
      r := clz(0x100)
      if iszero(eq(r, 247)) {
        revert(0, 0)
      }

      // CLZ(2^128) = 127
      r := clz(shl(128, 1))
      if iszero(eq(r, 127)) {
        revert(0, 0)
      }

      // CLZ(2^255) = 0
      r := clz(shl(255, 1))
      if iszero(eq(r, 0)) {
        revert(0, 0)
      }

      // CLZ(type(uint256).max) = 0
      r := clz(not(0))
      if iszero(eq(r, 0)) {
        revert(0, 0)
      }
    }
  }

  function executeG1BLSPrecompiles() external {
    bytes memory returnData;

    /// BLS12_G1ADD 0x0b precompile address
    (, returnData) = address(0x0b).staticcall(
      hex"0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee00000000000000000000000000000000177b39d2b8d31753ee35033df55a1f891be9196aec9cd8f512e9069d21a8bdbf693bd2e826e792cd12cb554287adf4ca0000000000000000000000000000000003c0f5770509862f754fc474cb163c41790d844f52939e2dec87b97c2a707831a4043ab47014d501f67862e95842ba5a"
    );
    require(returnData.length == 0, "BLS12_G1ADD test failed");

    (, returnData) = address(0x0b).staticcall(
      hex"0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee"
    );
    require(returnData.length == 128, "BLS12_G1ADD test failed");

    /// BLS12_G1MSM 0x0c precompile address
    (, returnData) = address(0x0c).staticcall(
      hex"0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee0000000000000000000000000000000000000000000000000000000000000002"
    );
    require(returnData.length == 128, "BLS12_G1MSM test failed");

    (, returnData) = address(0x0c).staticcall(
      hex"0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee0000000000000000000000000000000000000000000000000000000000000001"
    );
    require(returnData.length == 128, "BLS12_G1MSM test failed");
  }

  function executeG2BLSPrecompiles() external {
    bytes memory returnData;

    /// BLS12_G2ADD 0x0d precompile address
    (, returnData) = address(0x0d).staticcall(
      hex"00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a914500000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a9145"
    );
    require(returnData.length == 256, "BLS12_G2ADD test failed");

    (, returnData) = address(0x0d).staticcall(
      hex"00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a9145000000000000000000000000000000000b2c619263417e8f6cffa2e53261cb8cf5fbbabb9e6f4188aeaabe50d434a0489b6cccd2b65b4d1393a26911021baffa00000000000000000000000000000000007bcd4156af7ebe5e2f6ac63db859c9f42d5f11682792a0de2ec1db76648c0c98fdd8a82cf640bdcd309901afd4f57000000000000000000000000000000000153a9002d117a518b2c1786f9e8b95b00e936f3f15302a27a16d7f2f8fc48ca834c0cf4fce456e96d72f01f252f4d084000000000000000000000000000000001091fc53100190db07ec2057727859e65da996f6792ac5602cb9dfbc3ed4a5a67d6b82bd82112075ef8afc4155db2621"
    );
    require(returnData.length == 0, "BLS12_G2ADD test failed");

    /// BLS12_G2MSM 0x0e precompile address
    (, returnData) = address(0x0e).staticcall(
      hex"00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a91450000000000000000000000000000000000000000000000000000000000000001"
    );
    require(returnData.length == 256, "BLS12_G2MSM test failed");

    (, returnData) = address(0x0e).staticcall(
      hex"00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a9145000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a91450000000000000000000000000000000000000000000000000000000000000002"
    );
    require(returnData.length == 256, "BLS12_G2MSM test failed");
  }

  function executeBLSPairingAndMapPrecompiles() external {
    bytes memory returnData;
    /// BLS12_PAIRING_CHECK 0x0f precompile address
    (, returnData) = address(0x0f).staticcall(
      hex"0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a9145"
    );
    require(returnData.length == 32, "BLS12_PAIRING_CHECK test failed");

    (, returnData) = address(0x0f).staticcall(
      hex"0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a91450000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f560000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a9145"
    );
    require(returnData.length == 32, "BLS12_PAIRING_CHECK test failed");

    /// BLS12_MAP_FP_TO_G1 0x10 precompile address
    (, returnData) = address(0x10).staticcall(
      hex"00000000000000000000000000000000156c8a6a2c184569d69a76be144b5cdc5141d2d2ca4fe341f011e25e3969c55ad9e9b9ce2eb833c81a908e5fa4ac5f03"
    );
    require(returnData.length == 128, "BLS12_MAP_FP_TO_G1 test failed");

    /// BLS12_MAP_FP2_TO_G2 0x11 precompile address
    (, returnData) = address(0x11).staticcall(
      hex"0000000000000000000000000000000007355d25caf6e7f2f0cb2812ca0e513bd026ed09dda65b177500fa31714e09ea0ded3a078b526bed3307f804d4b93b040000000000000000000000000000000002829ce3c021339ccb5caf3e187f6370e1e2a311dec9b75363117063ab2015603ff52c3d3b98f19c2f65575e99e8b78c"
    );
    require(returnData.length == 256, "BLS12_MAP_FP2_TO_G2 test failed");
  }

  function executePrecompiles() external {
    bytes memory returnData;

    /// P256VERIFY - Valid input
    (, returnData) = address(0x100).staticcall(
      hex"bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca6050232ba3a8be6b94d5ec80a6d9d1190a436effe50d85a1eee859b8cc6af9bd5c2e184cd60b855d442f5b3c7b11eb6c4e0ae7525fe710fab9aa7c77a67f79e6fadd762927b10512bae3eddcfe467828128bad2903269919f7086069c8c4df6c732838c7787964eaac00e5921fb1498a60f4606766b3d9685001558d1a974e7341513e"
    );
    bytes32 validP256VerifyOutput = abi.decode(returnData, (bytes32));
    require(validP256VerifyOutput == bytes32(uint256(1)), "P256VERIFY test failed");

    /// P256VERIFY - Invalid input
    (, returnData) = address(0x100).staticcall(
      hex"bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023d45c5741946b2a137f59262ee6f5bc91001af27a5e1117a64733950642a3d1e8b329f479a2bbd0a5c384ee1493b1f5186a87139cac5df4087c134b49156847db2927b10512bae3eddcfe467828128bad2903269919f7086069c8c4df6c732838c7787964eaac00e5921fb1498a60f4606766b3d9685001558d1a974e7341513e"
    );
    require(returnData.length == 0, "P256VERIFY test failed");

    /// P256VERIFY - Invalid input
    (, returnData) = address(0x100).staticcall(
      hex"bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023d45c5740946b2a147f59262ee6f5bc90bd01ed280528b62b3aed5fc93f06f739b329f479a2bbd0a5c384ee1493b1f5186a87139cac5df4087c134b49156847db2927b10512bae3eddcfe467828128bad2903269919f7086069c8c4df6c732838c7787964eaac00e5921fb1498a60f4606766b3d9685001558d1a974e7341513e"
    );
    require(returnData.length == 0, "P256VERIFY test failed");

    /// POINT OF EVALUATION 0x0a precompile address
    (, returnData) = address(0x0a).staticcall(
      hex"01e798154708fe7789429634053cbf9f99b619f9f084048927333fce637f549b73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff000000001522a4a7f34e1ea350ae07c29c96c7e79655aa926122e95fe69fcbd932ca49e98f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7a62ad71d14c5719385c0686f1871430475bf3a00f0aa3f7b8dd99a9abc2160744faf0070725e00b60ad9a026a15b1a8c"
    );
    require(returnData.length == 64, "POINT OF EVALUATION test failed");

    (, returnData) = address(0x0a).staticcall(
      hex"0125681886f7d39de0938c4f5d2fb4d94abac545d2c51d242b930c6d667982e465cec67f404f8c81ef4ff3b08dc93a7f643c84e31d2cf39d094e9fbfcab15c9a00371c441de8235d2d858ade58d833ac6c5c9460fd369aea0c9918b6d007ed4787b470976941e342dba3361216d38797f94a249c89ab8fd29a9512b8cf0be7722eb93ca08dbc33f3bef4b8204f19098e8870e88b46732c642dc29b2a101fe309285300471f82de8adb40548918b5bcb7e8d9d126a3aa9d80f3559a39baa66a3d"
    );
    require(returnData.length == 0, "POINT OF EVALUATION test failed");
  }
}
