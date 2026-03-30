package circuits

import (
	"strings"

	"github.com/consensys/gnark/backend/solidity"
)

const (
	// lineaVerifierConstants contains additional constant declarations for the
	// Linea verifier contract.
	lineaVerifierConstants = `  bytes32 private immutable CHAIN_CONFIGURATION;`

	// lineaVerifierConstructor contains the constructor for the Linea verifier contract.
	lineaVerifierConstructor = `  /// @notice Constructor.
  /// @param _chainConfiguration The chain configuration parameters.
  constructor(ChainConfigurationParameter[] memory _chainConfiguration) {
    if (_chainConfiguration.length == 0) {
      revert ChainConfigurationNotProvided();
    }

    bytes32 chainConfigurationHash = _computeChainConfigurationHash(_chainConfiguration);

    CHAIN_CONFIGURATION = chainConfigurationHash;

    emit ChainConfigurationSet(chainConfigurationHash, _chainConfiguration);
  }`

	// lineaVerifierFunctions contains additional functions for the Linea verifier contract.
	lineaVerifierFunctions = `  /// @notice Compute the chain configuration hash.
  /// @param _chainConfiguration The chain configuration parameters.
  /// @return chainConfigurationHash The hash of the chain configuration.
  function _computeChainConfigurationHash(
    ChainConfigurationParameter[] memory _chainConfiguration
  ) internal pure returns (bytes32 chainConfigurationHash) {
    bytes memory mimcPayload;
    bytes32 value;
    for (uint256 i; i < _chainConfiguration.length; i++) {
      value = _chainConfiguration[i].value;

      bool firstBitIsZero;
      assembly {
        firstBitIsZero := iszero(shr(255, value))
      }

      if (firstBitIsZero) {
        mimcPayload = bytes.concat(mimcPayload, value);
      } else {
        bytes32 most;
        bytes32 least;

        assembly {
          most := shr(128, value)
          least := and(value, 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
        }
        mimcPayload = bytes.concat(mimcPayload, most, least);
      }
    }

    chainConfigurationHash = Mimc.hash(mimcPayload);
  }

  /// @notice Get the chain configuration.
  /// @return chainConfigurationHash The hash of the chain configuration.
  function getChainConfiguration() external view returns (bytes32) {
    return CHAIN_CONFIGURATION;
  }`
)

// LineaVerifierExportOptions returns the Solidity export options for generating
// the Linea verifier contract with chain configuration support. These options
// configure gnark's Solidity template with:
//   - Pragma version 0.8.33
//   - Imports for Mimc library and IPlonkVerifier interface
//   - IPlonkVerifier interface implementation declaration
//   - CHAIN_CONFIGURATION immutable variable
//   - Constructor accepting ChainConfigurationParameter array
//   - Chain configuration hash computation and getter functions
//
// Note: Import paths are relative to contracts/src/verifiers/. If the generated
// file is placed elsewhere, the import paths may need adjustment.
func LineaVerifierExportOptions() []solidity.ExportOption {
	return []solidity.ExportOption{
		solidity.WithPragmaVersion(solidityPragmaVersion),
		solidity.WithImport(strings.NewReader(`import { Mimc } from "../libraries/Mimc.sol";`)),
		solidity.WithImport(strings.NewReader(`import { IPlonkVerifier } from "./interfaces/IPlonkVerifier.sol";`)),
		solidity.WithInterface(strings.NewReader("IPlonkVerifier")),
		solidity.WithConstants(strings.NewReader(lineaVerifierConstants)),
		solidity.WithConstructor(strings.NewReader(lineaVerifierConstructor)),
		solidity.WithFunctions(strings.NewReader(lineaVerifierFunctions)),
	}
}
