# Linea tokens

### Build

```shell
$ forge build
```

### Test

```shell
$ forge test
```

### Deployment Process

#### 1. Prerequisites

- Install dependencies and build the project:
  ```sh
  forge build
  ```

#### 2. Deployment Scripts

##### a. Deploy LineaToken (L1)

- **Script:** `script/DeployLineaToken.s.sol`
- **Required env variables:**
  - `PROXY_ADMIN_OWNER_ADDRESS`: Address that will be assigned as the admin of the ProxyAdmin (can upgrade or change the implementation).
  - `TOKEN_ADMIN_ADDRESS`: Address that will be assigned as the admin of the token contract (can perform admin actions on the token).
  - `MINTER_ADDRESS`: Address that will be allowed to mint new tokens.
  - `L1_MESSAGE_SERVICE_ADDRESS`: Address of the message service contract on L1, used for cross-chain messaging.
  - `L2_LINEA_TOKEN_ADDRESS`: Address of the deployed LineaToken contract on L2.
  - `TOKEN_NAME`: The name of the token.
  - `TOKEN_SYMBOL`: The symbol of the token.
  - `PRIVATE_KEY`: The private key of the deployer account, used to sign and send transactions.
  - `EXPLORER_API_KEY`: API key for the block explorer (e.g., Etherscan) to enable contract verification.
  - `EXPLORER_API_URL`: The API endpoint for the block explorer (e.g., Etherscan) used for contract verification.
  - `RPC_URL`: The RPC endpoint URL for the target blockchain network.
- **Command:**
  ```sh
  forge script script/DeployLineaToken.s.sol --broadcast --private-key $PRIVATE_KEY --verifier-api-key $EXPLORER_API_KEY --verifier-url $EXPLORER_API_URL --rpc-url $RPC_URL --verify
  ```


  --constructor-args 0x \
eth:0x9EcE20C1878E4D67a0a8bA730f3F990a031B00a7

  forge verify-contract \
  --chain-id 1 \
  --compiler-version v0.8.30+commit.73712a01 \
  --num-of-optimizations 10000000 \
  0xC28dDc3F0d86cAAc8bBCa5EC0fF0D394f12F10c3 \
  "lib/openzeppelin-contracts/contracts/proxy/transparent/ProxyAdmin.sol:ProxyAdmin" \
  --compilation-profile 

##### b. Deploy L2LineaToken (L2)

- **Script:** `script/DeployL2LineaToken.s.sol`
- **Required env variables:**
  - `PROXY_ADMIN_OWNER_ADDRESS`: Address that will be assigned as the admin of the ProxyAdmin (can upgrade or change the implementation).
  - `TOKEN_ADMIN_ADDRESS`: Address that will be assigned as the admin of the token contract (can perform admin actions on the token).
  - `LINEA_CANONICAL_TOKEN_BRIDGE`: Address of the canonical token bridge contract on L2, used for bridging tokens between L1 and L2.
  - `LINEA_MESSAGE_SERVICE_ADDRESS`: Address of the message service contract on L2, used for cross-chain messaging.
  - `L1_LINEA_TOKEN_ADDRESS`: Address of the deployed LineaToken contract on L1.
  - `TOKEN_NAME`: The name of the token.
  - `TOKEN_SYMBOL`: The symbol of the token.
  - `PRIVATE_KEY`: The private key of the deployer account, used to sign and send transactions.
  - `EXPLORER_API_KEY`: API key for the block explorer (e.g., Lineascan) to enable contract verification.
  - `EXPLORER_API_URL`: The API endpoint for the block explorer (e.g., Lineascan) used for contract verification.
  - `RPC_URL`: The RPC endpoint URL for the target blockchain network.
- **Command:**
  ```sh
  forge script script/DeployL2LineaToken.s.sol --broadcast --private-key $PRIVATE_KEY --verifier-api-key $EXPLORER_API_KEY --verifier-url $EXPLORER_API_URL --rpc-url $RPC_URL --verify
  ```

##### c. Deploy TokenAirdrop

- **Script:** `script/DeployTokenAirdrop.s.sol`
- **Required env variables:**
  - `TOKEN_ADDRESS`: Address of the token contract to be distributed in the airdrop.
  - `OWNER_ADDRESS`: Address that will be assigned as the owner of the airdrop contract.
  - `CLAIM_END`: Unix timestamp (in seconds) after which claiming tokens from the airdrop is no longer possible.
  - `PRIMARY_FACTOR_ADDRESS`: Address of the contract providing the primary factor for airdrop calculations.
  - `PRIMARY_CONDITIONAL_MULTIPLIER_ADDRESS`: Address of the contract providing the primary conditional multiplier for airdrop calculations.
  - `SECONDARY_FACTOR_ADDRESS`: Address of the contract providing the secondary factor for airdrop calculations.
  - `PRIVATE_KEY`: The private key of the deployer account, used to sign and send transactions.
  - `EXPLORER_API_KEY`: API key for the block explorer (e.g., Lineascan) to enable contract verification.
  - `EXPLORER_API_URL`: The API endpoint for the block explorer (e.g., Lineascan) used for contract verification.
  - `RPC_URL`: The RPC endpoint URL for the target blockchain network.
- **Command:**
  ```sh
  forge script script/DeployTokenAirdrop.s.sol --broadcast --private-key $PRIVATE_KEY --verifier-api-key $EXPLORER_API_KEY --verifier-url $EXPLORER_API_URL --rpc-url $RPC_URL --verify
  ```
