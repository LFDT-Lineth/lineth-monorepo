// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./MemoryOperations2.sol";

contract MemoryOperations1 {
    event Log(bytes data);
    event returndatasizeCheck(uint256 size, string desc);

    uint256 seed;
    uint256 currentSampleNumber;
    event RandomExec(string desc);

    uint constant NUMBER_TESTING_CASES = 6;
    uint256 constant MAX_OFFSET = 16384;
    uint256 constant MAX_SIZE = 16384;
    uint256 constant MAX_VALUE = 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff;

    // Executes n random memory operations
    function execRandomMemoryOperations(uint256 n, uint256 _seed) public {
        /* Set seed to a desired value and reset currentSampleNumber to 1 so as the same seed
        generates the same sequence of random numbers / the same sequence of
        random called functions */
        seed = _seed;
        currentSampleNumber = 1;
        uint256 prn;

        for (uint256 i = 0; i < n; i++) {
            prn = getPRN(NUMBER_TESTING_CASES);
            if (prn == 0) {
                emit RandomExec("msizeExec");
                msizeExec();
            } else if (prn == 1) {
                emit RandomExec("mloadExec");
                mloadExec(getPRN(MAX_OFFSET));
            } else if (prn == 2) {
                emit RandomExec("mstoreExec");
                mstoreExec(getPRN(MAX_OFFSET), getPRN(MAX_VALUE));
            } else if (prn == 3) {
                emit RandomExec("mstore8Exec");
                mstore8Exec(getPRN(MAX_OFFSET), uint8(getPRN(MAX_VALUE)));
            } else if (prn == 4) {
                emit RandomExec("logExec");
                logExec();
            } else if (prn == 5) {
                emit RandomExec("keccak256Exec");
                keccak256Exec(getPRN(MAX_OFFSET), getPRN(MAX_SIZE));
            }
        }
    }

    // Support function to generate random numbers
    function getPRN(uint256 maxValue) private returns(uint256) {
        bytes32 h = keccak256(abi.encodePacked(seed, currentSampleNumber));
        currentSampleNumber += 1;
        uint256 n = uint256(h) % maxValue;
        return n;
    }

    // Testing case 0
    function msizeExec() public pure returns (uint256) {
        uint256 size;
        assembly {
            // Retrieve the current size of the memory in use
            size := msize()
        }
        return size;
    }

    // Testing case 1
    function mloadExec(uint256 offset) public pure returns (uint256) {
        uint256 value;
        assembly {
            // Load a 32-byte word from memory at the specified offset
            value := mload(offset)
        }
        return value;
    }

    // Testing case 2
    function mstoreExec(uint256 offset, uint256 value) public pure {
        // msizeExec();
        assembly {
            // Store the 32-byte value at the specified offset
            mstore(offset, value)
        }
        // assert(mloadExec(offset) == value);
        // msizeExec();
    }

    // Testing case 3
    function mstore8Exec(uint256 offset, uint8 value) public pure {
        assembly {
            // Store the 8-bit value at the specified offset
            mstore8(offset, value)
        }
    }

    // Testing case 4
    function logExec() public {
        // Log some data
        bytes memory data = new bytes(64);
        data = hex"aaaa567890abcdef0123456789abcdef01234567890abcdef0123456789abcde1234567890abcdef0123456789abcdef01234567890abcdef0123456789affff";
        emit Log(data);
    }

    // Testing case 5
    function keccak256Exec(uint256 offset, uint256 size) public pure returns (bytes32) {
        bytes32 hash;
        assembly {
            // Compute the KECCAK256 hash of the input data
            hash := keccak256(offset, size)
        }
        return hash;
    }

    // Testing case final 1 (not included in the random selection)
    function returndatacopyExecAfterReturn(address addressMO2) public {
        emit RandomExec("returndatacopyExec (after return)");
        MemoryOperations2(addressMO2).returnExec(0);
        uint256 rds;
        assembly {
            rds := returndatasize()
        }
        uint256 destOffset = getPRN(MAX_OFFSET);
        uint256 offset = getPRN(rds - 1); // This will fail if rds < 2
        uint256 size = getPRN(rds - offset);
        assembly {
            // offset + size has to be <= than the return data size
            // TODO: we should target offset/size pairs not multiple of 16, small, large etc
            returndatacopy(destOffset, offset, size)
        }
    }

    // Testing case final 2 (not included in the random selection)
    function returndatacopyExecAfterRevert(address addressMO2) public {
        emit RandomExec("returndatacopyExec (after revert)");
        try MemoryOperations2(addressMO2).revertExec(0) {

        } catch {

        }
        uint256 rds;
        assembly {
            rds := returndatasize()
        }
        uint256 destOffset = getPRN(MAX_OFFSET);
        uint256 offset = getPRN(rds - 1); // This will fail if rds < 2
        uint256 size = getPRN(rds - offset);
        assembly {
            // offset + size has to be <= than the return data size
            // TODO: we should target offset/size pairs not multiple of 16, small, large etc
            returndatacopy(destOffset, offset, size)
        }
        /* TODO: this point of the execution is executed (after the try/catch) but
        it cannot be reached via the Remix debugger, so it is still not clear if this test
        case works as expected. Clarify it.
        */
    }
}
