// This has been compiled on REMIX without the optimization and stored as contracts/local-deployments-artifacts/static-artifacts/LondonEvmCodes.json
// If you copy the bytecode from the verbatim_0i_0o section and open in https://evm.codes you can step through the entire execution.
// Compiler: 0.8.19, no optimizations and London EVM Version

object "DynamicBytecode" {
    code {
        datacopy(0x00, dataoffset("runtime"), datasize("runtime"))
        return(0x00, datasize("runtime"))
    }

    object "runtime" {
        code {
            switch selector()
            case 0xa378ff3e // executeAll() 
            {
                doExternalCallsAndMStore8()
                executeOpcodes()
            }
           
            default {
                // if the function signature sent does not match any
                revert(0, 0)
            }

            function doExternalCallsAndMStore8(){
                
                // Using a random function on an EOA for all calls other than the embedded staticcall in the verbatim code to the precompile
                // - should be a successful call for all.

                let callSelector := 0xfed44325

                // Load the free memory pointer
                let ptr := mload(0x40)

                // Store the selector in memory at the pointer location
                mstore(ptr, callSelector)

                // Perform the call
                let success := call(
                    gas(),         // Forward all available gas
                    0x55,          // Random address
                    0,             // No Ether to transfer
                    ptr,           // Pointer to input data (selector)
                    0x04,          // Input size (4 bytes for the selector)
                    0,             // No output data
                    0              // No output size
                )

                // Handle the call result
                if iszero(success) {
                    revert(0, 0)  // Revert with no message if the call fails
                }

               success := callcode(
                    gas(),         // Forward all available gas
                    0x55,          // Random address
                    0,             // No Ether to transfer
                    ptr,           // Pointer to input data (selector)
                    0x04,          // Input size (4 bytes for the selector)
                    0,             // No output data
                    0              // No output size
                )

                // Handle the call result
                if iszero(success) {
                    revert(0, 0)  // Revert with no message if the call fails
                }

               success := delegatecall(
                    gas(),         // Forward all available gas
                    0x55,          // Random address
                    ptr,           // Pointer to input data (selector)
                    0x04,          // Input size (4 bytes for the selector)
                    0,             // No output data
                    0              // No output size
                )

                // Handle the call result
                if iszero(success) {
                    revert(0, 0)  // Revert with no message if the call fails
                }

                ptr := add(ptr,0x4)

                // Make sure MSTORE8 opcode is called
                mstore8(ptr,0x1234567812345678)
            }

            function executeOpcodes() {
                // Verbatim bytecode to do most of Shanghai including the precompile and control flow opcodes:
                verbatim_0i_0o(hex"5F506460006000f3600052426005601b5ff550600060006000F050600060006000600060006000A460006000600060006000A36000600060006000A2600060006000A160006000A0585059505A50426000556000545060004050415042504350445045504650475048507300000000000000000000000000000000000000003F506000600060003E6000600060007300000000000000000000000000000000000000003C3D507300000000000000000000000000000000000000003B5060016001016001026003036001046001056001066001076001600108600160010960020160030A60010B600810600A11600112600113600114156001166001176001181960161A60011B60011C60011D506000600020303132333450505050503635600060003738604051600081016000600083393A50505050607e50617e0150627e012350637e01234550647e0123456750657e012345678950667e0123456789AB50677e0123456789ABCD50687e0123456789ABCDEF50697e0123456789ABCDEF01506a7e0123456789ABCDEF0123506b7e0123456789ABCDEF012345506c7e0123456789ABCDEF01234567506d7e0123456789ABCDEF0123456789506e7e0123456789ABCDEF0123456789AB506f7e0123456789ABCDEF0123456789ABCD50707e0123456789ABCDEF0123456789ABCDEF50717e0123456789ABCDEF0123456789ABCDEF0150727e0123456789ABCDEF0123456789ABCDEF012350737e0123456789ABCDEF0123456789ABCDEF01234550747e0123456789ABCDEF0123456789ABCDEF0123456750757e0123456789ABCDEF0123456789ABCDEF012345678950767e0123456789ABCDEF0123456789ABCDEF0123456789AB50777e0123456789ABCDEF0123456789ABCDEF0123456789ABCD50787e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF50797e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF01507a7e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123507b7e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF012345507c7e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF01234567507d7e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789507e0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCD507f0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f5050505050505050505050505050505050")
            }

            // Return the function selector: the first 4 bytes of the call data
            function selector() -> s {
                s := div(calldataload(0), 0x100000000000000000000000000000000000000000000000000000000)
            }
        }
    }
}