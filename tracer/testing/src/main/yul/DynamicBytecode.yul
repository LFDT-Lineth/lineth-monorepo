object "DynamicBytecode" {
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
        }
    }
}