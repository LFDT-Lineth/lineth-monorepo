// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract LogEmitter {
    // Emit a log with 0 topics
    function log0(bytes calldata data) external {
        assembly {
            log0(add(data.offset, 0x20), data.length)
        }
    }

    // Emit a log with 1 topic
    function log1(bytes32 t1, bytes calldata data) external {
        assembly {
            log1(add(data.offset, 0x20), data.length, t1)
        }
    }

    // Emit a log with 2 topics
    function log2(bytes32 t1, bytes32 t2, bytes calldata data) external {
        assembly {
            log2(add(data.offset, 0x20), data.length, t1, t2)
        }
    }

    // Emit a log with 3 topics
    function log3(bytes32 t1, bytes32 t2, bytes32 t3, bytes calldata data) external {
        assembly {
            log3(add(data.offset, 0x20), data.length, t1, t2, t3)
        }
    }

    // Emit a log with 4 topics
    function log4(bytes32 t1, bytes32 t2, bytes32 t3, bytes32 t4, bytes calldata data) external {
        assembly {
            log4(add(data.offset, 0x20), data.length, t1, t2, t3, t4)
        }
    }

    // Emit multiple logs with the same topic (for L2L1 log limit testing)
    function emitMultipleLogs(uint256 count, bytes32 topic) external {
        for (uint256 i = 0; i < count; i++) {
            assembly {
                log1(0, 0, topic)
            }
        }
    }
}
