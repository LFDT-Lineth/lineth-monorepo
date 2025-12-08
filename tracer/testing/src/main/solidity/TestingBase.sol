// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

/**
 * @notice Shared base contract for functions and events relating to call execution and contract creation.
 */
abstract contract TestingBase {
    /// @dev The event indicating call status, the index in the collection and the address being called.
    /// @param isSuccess The success bool.
    /// @param callIndex The index of the call in the collection.
    /// @param destination The destination contract address where the call is made to.
    event CallExecuted(
        bool indexed isSuccess,
        uint256 callIndex,
        address indexed destination,
        address indexed executedFrom
    );

    /// @dev The contract created event;
    /// @param contractAddress The contract address.
    event ContractCreated(address contractAddress);

    /// @dev The contract destroyed event;
    /// @param contractAddress The contract address.
    event ContractDestroyed(address contractAddress);

    /// @dev The event for writing storage;
    /// @param contractAddress is the contract addresss, x is the storage key and y is the storage value.
    /// Event signature: 33d8dc4a860afa0606947f2b214f16e21e7eac41e3eb6642e859d9626d002ef6
    event Write(address contractAddress, uint256 x, uint256 y);

    /// @dev The event for reading storage;
    /// @param contractAddress is the contract addresss, x is the storage key and y is the storage value.
    /// the event will be generated after y is read as the value stored at x.
    /// Event signature: c2db4694c1ec690e784f771a7fe3533681e081da4baa4aa1ad7dd5c33da95925
    event Read(address contractAddress, uint256 x, uint256 y);

    event EventReadFromStorage(address contractAddress, uint256 x);

    /// @dev The Call types for external calls.
    /// @dev Default is delegate call (value==0), so be aware.
    enum CallType {
        DELEGATE_CALL,
        CALL,
        STATIC_CALL
    }

    /**
     * @notice Struct describing an external call to execute.
     * _contract The contract address to call.
     * _calldata The function signature and parameters encoded to send to the _contract address.
     * _gasLimit The gas to send with the call - 0 sends all.
     * _callType The call type to use - call, static or delegatecall.
     */
    struct ContractCall {
        address _contract;
        bytes _calldata;
        uint256 _gasLimit;
        uint256 _value;
        CallType _callType;
    }

    /**
     * @notice Iterates over and executes `ContractCall`s.
     * @dev CallExecuted is emitting indicating success or failure of the particular call.
     * @dev If the gas limit is zero gasleft() will be used to send all the gas.
     * @param _contractCalls The calls to encode and store.
     */
    function executeCalls(
        ContractCall[] memory _contractCalls
    ) public payable virtual {
        for (uint256 i; i < _contractCalls.length; ) {
            uint256 gasToUse = _contractCalls[i]._gasLimit == 0
                ? gasleft()
                : _contractCalls[i]._gasLimit;

            /// Handle failures by emitting events.
            /// @dev function is retrieved and executed - some params aren't used on some of the functions.
            bool success = getCallFunction(_contractCalls[i]._callType)(
                _contractCalls[i]._contract,
                _contractCalls[i]._calldata,
                gasToUse,
                _contractCalls[i]._value
            );

            emit CallExecuted(
                success,
                i,
                _contractCalls[i]._contract,
                address(this)
            );

            unchecked {
                i++;
            }
        }
    }

    /**
     * @notice Reads data from a contract's code as bytes, converts to external calls and executes them.
     * @param _contractWithSteps The address of the contract containing data to read.
     */
    function getContractBasedStepsAndExecute(
        address _contractWithSteps
    ) public payable {
        ContractCall[] memory steps = abi.decode(
            readContractAsData(_contractWithSteps, 0),
            (ContractCall[])
        );

        executeCalls(steps);
    }

    /**
     * @notice Determines call function type based on enum value.
     * @param _callType Enum value for the different external call types.
     * @return function to execute.
     */
    function getCallFunction(
        CallType _callType
    )
    internal
    pure
    returns (
        function(address, bytes memory, uint256, uint256) returns (bool)
    )
    {
        if (_callType == CallType.DELEGATE_CALL) {
            return doDelegateCall;
        }

        if (_callType == CallType.CALL) {
            return doCall;
        }

        return doStaticCall;
    }

    /**
     * @notice Executes a delegatecall.
     * @param _address The address to call.
     * @param _calldata The calldata to pass for function selection and parameters.
     * @param _gas The gas to use.
     * @return The success or failure boolean.
     */
    function doDelegateCall(
        address _address,
        bytes memory _calldata,
        uint256 _gas,
        uint256
    ) internal returns (bool) {
        (bool success, ) = _address.delegatecall{gas: _gas}(_calldata);
        return success;
    }

    /**
     * @notice Executes a normal external call.
     * @param _address The address to call.
     * @param _calldata The calldata to pass for function selection and parameters.
     * @param _gas The gas to use.
     * @param _value The value to pass to the contract.
     * @return The success or failure boolean.
     */
    function doCall(
        address _address,
        bytes memory _calldata,
        uint256 _gas,
        uint256 _value
    ) internal returns (bool) {
        (bool success, ) = _address.call{gas: _gas, value: _value}(_calldata);
        return success;
    }

    /**
     * @notice Executes a normal static call.
     * @param _address The address to call.
     * @param _calldata The calldata to pass for function selection and parameters.
     * @param _gas The gas to use.
     * @return The success or failure boolean.
     */
    function doStaticCall(
        address _address,
        bytes memory _calldata,
        uint256 _gas,
        uint256
    ) internal view returns (bool) {
        (bool success, ) = _address.staticcall{gas: _gas}(_calldata);
        return success;
    }

    /**
     * @notice Encodes `ContractCall`s and writes `data` into the bytecode of a storage contract and returns its address.
     * @param _contractCalls The calls to encode and store.
     * @return contractAddress The address of the newly deployed contract containing the data.
     */
    function encodeCallsToContract(
        ContractCall[] calldata _contractCalls
    ) public returns (address contractAddress) {
        contractAddress = writeDataAsContract(abi.encode(_contractCalls));
    }

    /**
     * @notice Writes `data` into the bytecode of a storage contract and returns its address.
     * @dev sourced from Solady (https://github.com/vectorized/solady/blob/main/src/utils/SSTORE2.sol)
     * @param _data The data to store.
     * @return pointer The address of the newly deployed contract containing the data.
     */
    function writeDataAsContract(
        bytes memory _data
    ) public virtual returns (address pointer) {
        /// @solidity memory-safe-assembly
        assembly {
            let n := mload(_data) // Let `l` be `n + 1`. +1 as we prefix a STOP opcode.
        /**
         * ---------------------------------------------------+
         * Opcode | Mnemonic       | Stack     | Memory       |
         * ---------------------------------------------------|
         * 61 l   | PUSH2 l        | l         |              |
         * 80     | DUP1           | l l       |              |
         * 60 0xa | PUSH1 0xa      | 0xa l l   |              |
         * 3D     | RETURNDATASIZE | 0 0xa l l |              |
         * 39     | CODECOPY       | l         | [0..l): code |
         * 3D     | RETURNDATASIZE | 0 l       | [0..l): code |
         * F3     | RETURN         |           | [0..l): code |
         * 00     | STOP           |           |              |
         * ---------------------------------------------------+
         * @dev Prefix the bytecode with a STOP opcode to ensure it cannot be called.
             * Also PUSH2 is used since max contract size cap is 24,576 bytes which is less than 2 ** 16.
             */
        // Do a out-of-gas revert if `n + 1` is more than 2 bytes.
            mstore(
                add(_data, gt(n, 0xfffe)),
                add(0xfe61000180600a3d393df300, shl(0x40, n))
            )
        // Deploy a new contract with the generated creation code.
            pointer := create(0, add(_data, 0x15), add(n, 0xb))
            if iszero(pointer) {
                mstore(0x00, 0x30116425) // `DeploymentFailed()`.
                revert(0x1c, 0x04)
            }
            mstore(_data, n) // Restore the length of `data`.
        }
    }

    /**
     * @notice Reads data from a contract's code as bytes memory.
     * @dev The start parameter for the most part is 0 if the entire data is required.
     * @dev sourced from Solady (https://github.com/vectorized/solady/blob/main/src/utils/SSTORE2.sol)
     * @param _pointer The address of the contract containing data to read.
     * @param _start The offset of the data to read.
     */
    function readContractAsData(
        address _pointer,
        uint256 _start
    ) public view virtual returns (bytes memory data) {
        /// @solidity memory-safe-assembly
        assembly {
            data := mload(0x40)
            let n := and(sub(extcodesize(_pointer), 0x01), 0xffffffffff)
            extcodecopy(_pointer, add(data, 0x1f), _start, add(n, 0x21))
            mstore(data, mul(sub(n, _start), lt(_start, n))) // Store the length.
            mstore(0x40, add(data, add(0x40, mload(data)))) // Allocate memory.
        }
    }

    /**
     * @notice Deploys a contract with create 2.
     * @param _salt The salt for creating the contract.
     * @param _bytecode The bytecode to use in creation.
     * @return addr The new contract address.
     */
    function deployWithCreate2(
        bytes32 _salt,
        bytes memory _bytecode
    ) public payable returns (address addr) {
        assembly {
            let value := callvalue()
            addr := create2(
                value,
                add(_bytecode, 0x20),
                mload(_bytecode),
                _salt
            )
                if iszero(addr) {
                revert(0, 0)
            }
        }

        emit ContractCreated(addr);
    }

    /**
     * @notice Deploys a contract with create 2 and can end with a revert on demand.
     * @param _salt The salt for creating the contract.
     * @param _bytecode The bytecode to use in creation.
     * @param _endWithRevert Trigger a revert after contract creation.
     * @return addr The new contract address.
     */
    function deployWithCreate2_withRevertTrigger(
        bytes32 _salt,
        bytes memory _bytecode,
        bool _endWithRevert
    ) public payable returns (address addr) {
        assembly {
            let value := callvalue()
            addr := create2(
                value,
                add(_bytecode, 0x20),
                mload(_bytecode),
                _salt
            )
            if iszero(addr) {
                revert(0, 0)
            }
        }

        emit ContractCreated(addr);
        if (_endWithRevert) {
            revert();
        }
    }

    /**
     * @notice Deploys a contract with create 2, sends a value and never reverts. Event only emitted if address is not 0.
     * @param _salt The salt for creating the contract.
     * @param _bytecode The bytecode to use in creation.
     * @param _value The value sent during the creation.
     * @return addr The new contract address.
     */
    function deployWithCreate2_withValueNoRevert(
        bytes32 _salt,
        bytes memory _bytecode,
        uint256 _value
    ) public payable returns (address addr) {
        assembly {
            addr := create2(
                _value,
                add(_bytecode, 0x20),
                mload(_bytecode),
                _salt
            )
        }

        if (addr != address(0)) {
            emit ContractCreated(addr);
        }
    }

    /**
 * @notice Deploys a contract with create 2, sends a value and can revert. Event only emitted if address is not 0.
     * @param _salt The salt for creating the contract.
     * @param _bytecode The bytecode to use in creation.
     * @param _value The value sent during the creation.
     * @return addr The new contract address.
     */
    function deployWithCreate2_withValueAndRevert(
        bytes32 _salt,
        bytes memory _bytecode,
        uint256 _value
    ) public payable returns (address addr) {
        assembly {
            addr := create2(
                _value,
                add(_bytecode, 0x20),
                mload(_bytecode),
                _salt
            )
            if iszero(addr) {
                revert(0, 0)
            }
        }

        if (addr != address(0)) {
            emit ContractCreated(addr);
        }
    }

    /**
     * @notice Predetermines a Create2 address.
     * @param _deployerAddress The deploying contract's address.
     * @param _bytecode The bytecode to use in creation.
     * @param _salt The salt being used.
     * @return addr The expected contract address.
     */
    function calculateCreate2Address(
        address _deployerAddress,
        bytes memory _bytecode,
        bytes32 _salt
    ) public pure returns (address addr) {
        addr = address(
            uint160(
                uint256(
                    keccak256(
                        abi.encodePacked(
                            bytes1(0xff),
                            _deployerAddress,
                            _salt,
                            keccak256(_bytecode)
                        )
                    )
                )
            )
        );
    }

    /**
     * @notice Selfdestructs and sends remaining ETH to a payable address.
     * @dev Keep in mind you need to compile and target London EVM version - this doesn't work for repeat addresses on Cancun etc.
     * @param _fundAddress The deploying contract's address.
     */
    // 0x3f5a0bdd0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4 (replace 5b38da6a701c568545dcfcb03fcb875f56beddc4)
    function selfDestruct(address payable _fundAddress) public {
        emit ContractDestroyed(address(this));
        selfdestruct(_fundAddress);
    }
}
