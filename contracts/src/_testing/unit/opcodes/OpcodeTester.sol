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

    // 0x0010 - 0x001D
    for (uint16 i = 0x0010; i <= 0x001D; i += 0x0001) {
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
}
