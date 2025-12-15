object "StateManagerSnippets" {
    code {
        // return the bytecode of the contract
        datacopy(0x00, dataoffset("runtime"), datasize("runtime"))
        return(0x00, datasize("runtime"))
    }

    object "runtime" {
        code {
            switch selector()
            case 0xa770741d // Write()
            {
               write()
            }

            case 0x97deb47b // Read()
            {
                // store the ID to memory at 0x00
                mstore(0x00, sload(0x00))

                return (0x00,0x20)
            }

            case 0xacf07154 // writeToStorage()
            {
                // Load the first argument (x) from calldata
                let x := calldataload(0x04)

                // Load the second argument (y) from calldata, 0x20 is 32 bytes
                let y := calldataload(0x24)

                // get the revertFlag
                let revertFlag := calldataload(0x44)

                // call the writeToStorage function
                writeToStorage(x, y, revertFlag)

                // log the call
                logValuesWrite(x, y)

                // check the revert flag, and if true, perform a revert
                if eq(revertFlag, 0x0000000000000000000000000000000000000000000000000000000000000001) {
                    revertWithError()
                }
            }

            case 0x2d97bf10 // readFromStorage()
            {
                // Load the first argument (x) from calldata
                let x := calldataload(0x04)

                // get the revertFlag
                let revertFlag := calldataload(0x24)

                // call the readFromStorage function
                let y := readFromStorage(x, revertFlag)

                // log the call
                logValuesRead(x, y)

                // check the revert flag, and if true, perform a revert
                if eq(revertFlag, 0x0000000000000000000000000000000000000000000000000000000000000001) {
                    revertWithError()
                }
            }

            case 0xeba7ff7f // selfDestruct()
            {
                // Load the first argument (recipient) from calldata
                let recipient := calldataload(0x04)

                // get the revertFlag
                let revertFlag := calldataload(0x24)

                // log the call before self destructing
                logSelfDestruct(recipient)

                // call the self-destruct function if the revert is not present
                // not the best way to revert, but the self destruct does not allow for a revert afterwards
                // unless the revert flag is pushed outside, after the end of the contract call
                if eq(revertFlag, 0x0000000000000000000000000000000000000000000000000000000000000000) {
                    selfDestruct(recipient, revertFlag)
                }

                // check the revert flag, and if true, perform a revert
                if eq(revertFlag, 0x0000000000000000000000000000000000000000000000000000000000000001) {
                    revertWithError()
                }
            }

            case 0x2b261e94 // transferTo(), transfer to a specific address
            {
                // Load the recipient address from calldata
                let recipient := calldataload(0x04)

                // get the amount
                let amount := calldataload(0x24)

                // get the revertFlag
                let revertFlag := calldataload(0x44)

                // address of the sender contract
                let senderAddress := address()

                // perform the transfer
                transferTo(recipient, amount, revertFlag)

                // log the call
                logTransfer(senderAddress, recipient, amount)

                // check the revert flag, and if true, perform a revert
                if eq(revertFlag, 0x0000000000000000000000000000000000000000000000000000000000000001) {
                    revertWithError()
                }
            }

            case 0x3ecfd51e {
                // I will use 0xf1f1f1f1 as hardcoded calldata for the call opcode in case of a transfer
                logReceive()
                receiveETH()
            }

            case 0xffffffff {
                // I will use 0xf1f1f1f1 as hardcoded calldata for the call opcode in case of a transfer
            }

            default {
                // if the function signature sent does not match any
                // of the contract functions, revert
                revert(0, 0)
            }

            function write() {
                // take no inputs, push the 1234...EF 32 byte integer onto the stack
                // add 0 as the slot key to the stack
                // store the key/value
                verbatim_0i_0o(hex"7f0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF600055")
            }

            // Function to log two uint256 values for the write storage operation, along with the contract address
           function logValuesWrite(x, y) {
                // Define an event signature to generate logs for storage operations
                let eventSignature := "Write(address,uint256,uint256)" // The signature is "Write(address,uint256,uint256)", signature length is 30 characters (thus 30 bytes)

                let memStart := mload(0x40) // get the free memory pointer
                mstore(memStart, eventSignature)

                let eventSignatureHash := keccak256(memStart, 30) // expected 33d8dc4a860afa0606947f2b214f16e21e7eac41e3eb6642e859d9626d002ef6

                // in the case of a delegate call, this will be the original that called the .Yul snippet.
                // For a regular call, it will be the address of the .Yul contract.
                let contractAddress := address()

                // call the inbuilt logging function
                log4(0x20, 0x60, eventSignatureHash, contractAddress, x, y)
           }
           // Function to log two uint256 values for the read storage operation, along with the contract address
           function logValuesRead(x, y) {
                // Define an event signature to generate logs for storage operations
                let eventSignatureHex := 0x5265616428616464726573732c75696e743235362c75696e7432353629202020
                // Above is the hex for "Read(address,uint256,uint256)", exact length is 29 characters, padded with 3 spaces (20 in hex)
                // if we do not pad at the end, the hex is stored in the wrong manner. The spaces will be disregarded when we hash.

                let memStart := mload(0x40) // get the free memory pointer
                mstore(memStart, eventSignatureHex)

                let eventSignatureHash := keccak256(memStart, 29) // 29 bytes is the string length, expected output c2db4694c1ec690e784f771a7fe3533681e081da4baa4aa1ad7dd5c33da95925

                // in the case of a delegate call, this will be the original that called the .Yul snippet.
                // For a regular call, it will be the address of the .Yul contract.
                let contractAddress := address()

                // call the inbuilt logging function
                log4(0x20, 0x60, eventSignatureHash, contractAddress, x, y)
           }

           // Function to log a transfer
           function logTransfer(sender, recipient, amount) {
                // Define an event signature to generate logs for storage operations
                let eventSignature := "PayETH(address,address,uint256)"

                let memStart := mload(0x40) // get the free memory pointer
                mstore(memStart, eventSignature)

                let eventSignatureHash := keccak256(memStart, 31) // 31 bytes is the string length, expected output 86486637435fcc400fa51609bdb9068db32be14298e016223d7b7ffdae7998ff


                // call the inbuilt logging function
                log4(0x20, 0x60, eventSignatureHash, sender, recipient, amount)
           }

            // Function to log the self destruction of the contract
           function logSelfDestruct(recipient) {
                // Define an event signature to generate logs for storage operations
                let eventSignature := "ContractDestroyed(address)"

                let memStart := mload(0x40) // get the free memory pointer
                mstore(memStart, eventSignature)

                let eventSignatureHash := keccak256(memStart, 26) // 26 bytes is the string length, expected output 3ab1d7915d663a46c292b8f01ac13567c748cff5213cb3652695882b5f9b2e0f

                // call the inbuilt logging function
                log2(0x20, 0x60, eventSignatureHash, recipient)
           }

           function logReceive() {
                // Define an event signature to generate a log when the .yul contract receives ETH
                let eventSignature := "RecETH(address,uint256)"

                let memStart := mload(0x40) // get the free memory pointer
                mstore(memStart, eventSignature)

                let eventSignatureHash := keccak256(memStart, 23) // 23 bytes is the string length, expected output e1b5c1e280a4d97847c2d5c3006bd406609f68889f3d868ed3250aa10a8629aa

                let currentAddress := address()
                let amount := callvalue()
                // call the inbuilt logging function
                log3(0x20, 0x60, eventSignatureHash, currentAddress, amount)
           }

            // 0xacf071542e5c4e6701a11ffbc7a8f8e63ef5fdc6871e5d1ee4e7bc956a8d23ac
            // function signature: first 4 bytes 0xacf07154
            // example of call: [["Addr",0xacf07154000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001,0,0,0]]
            // writeToStorage(uint256,uint256,bool) is the unhashed function signature
            function writeToStorage(x, y, revertFlag) {
                // Use verbatim to include raw bytecode for storing y at storage key x
                // 55 corresponds to SSTORE
                verbatim_2i_0o(hex"55", x, y)
            }

            // 0x2d97bf1001ed6a98a40186556bfeee30afccf7c13d4d24f6eb6e48b668210fc8
            // function signature: first 4 bytes 0x2d97bf10
            // example of call: [["Addr",0x2d97bf1000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001,0,0,0]]
            // readFromStorage(uint256,bool) is the unhashed function signature
            function readFromStorage(x, revertFlag) -> y{
                // Use verbatim to include raw bytecode for reading the value stored at x
                // 54 corresponds to SLOAD
                y := verbatim_1i_1o(hex"54", x)
            }


            // @notice Selfdestructs and sends remaining ETH to a payable address.
            // @dev Keep in mind you need to compile and target London EVM version - this doesn't work for repeat addresses on Cancun etc.
            // example of call: [["Addr",0xeba7ff7f0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc40000000000000000000000000000000000000000000000000000000000000001,0,0,0]]
            // (replace the argument 5b38da6a701c568545dcfcb03fcb875f56beddc4 with the intended recipient, as needed)
            // function signature selfDestruct(address,bool)—eba7ff7f626dacfd4408d6e720f444f37df3477e2719d8610a3837a6f8b9400e
            function selfDestruct(recipient, revertFlag) {
                // There is a weird bug with selfdestruct(recipient), so we replace that with verbatim
                // Use verbatim to include the selfdestruct opcode (0xff)
                verbatim_1i_0o(hex"ff", recipient)
            }

            // Return the function selector: the first 4 bytes of the call data
            function selector() -> s {
                s := div(calldataload(0), 0x100000000000000000000000000000000000000000000000000000000)
            }

            // Implementation of the require statement from Solidity
            function require(condition) {
                if iszero(condition) { revert(0, 0) }
            }

            // Check if the calldata has the correct number of params
            function checkParamLength(len) {
                require(eq(calldatasize(), add(4, mul(32, len))))
            }

            // Transfer ether to the caller address
            function transfer(amount) {
                if iszero(call(gas(), caller(), amount, 0, 0, 0, 0)) {
                    revert(0,0)
                }
            }

            // Transfer ether to the recipient address
            // function signature transferTo(address,uint256,bool)—hash 0x2b261e94b0c8d2a13b0379d0e6facd43b4bd12e1d1b944f2ee6288c0a278d838
            // function selector 0x2b261e94
            // call example, snippet address—function selector, recipient address, wei amount and revert flag
            // call example: [["addr",0x2b261e94000000000000000000000000746FfB8C87c9142519a144caB812495d2960a24b00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000,0,0,1]]
            function transferTo(recipient, amount, revertFlag) {
                // hardcoding f1f1f1f1 as custom calldata in case of a transfer
                let formedCallData := mload(0x40)
                let functionSelector := 0x3ecfd51e00000000000000000000000000000000000000000000000000000000
                mstore(formedCallData, functionSelector)
                let success := call(gas(), recipient, amount, formedCallData, 0x20, 0, 0)
                // Check if the transfer was successful
                if iszero(success) {
                    // Revert the transaction if the transfer failed
                    revert(0, 0)
                }
            }

            // Function to revert with an error message
            function revertWithError() {
            // Define the error message in hexadecimal
            let errorMessage := "Reverting"

            // Allocate memory for the error message
            let errorMessageSize := 0x20  // 32 bytes
            let errorMessageOffset := mload(0x40)  // Load the free memory pointer
            mstore(errorMessageOffset, errorMessage)

            // Revert with the error message
            revert(errorMessageOffset, errorMessageSize)
            }

            // 3ecfd51e90ed5312cd5bc47447919dde093173d93c6a98ab52ecf0cfc491e099
            // function selector 0x3ecfd51e
            // we need this empty function in order for the contract to be sucessfully deployed
            // using the java wrappers
            function receiveETH() {

            }
        }
    }
}
