# AddressFilter

[← Back to index](../README.md)

<br />

Deploys the AddressFilter contract used to filter addresses for forced transactions. The contract is initialized with the deployer as temporary `DEFAULT_ADMIN_ROLE`, precompiles set in the constructor, and the remaining filtered addresses applied via batched `setFilteredStatus` calls. Admin is then transferred to the Security Council. Filtered addresses (beyond precompiles) are read from the file at `ADDRESS_FILTER_FILE_PATH` (default: `contracts/addresses-filter.txt`).

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| L1_SECURITY_COUNCIL | registry\|env | address | L1 Security Council address (receives `DEFAULT_ADMIN_ROLE` after filtered addresses are applied). Read from the address registry on stable networks; env var used as fallback or on unregistered networks. |
| ADDRESS_FILTER_FILE_PATH | false | path | Absolute or relative path to the filtered addresses file. Defaults to `contracts/addresses-filter.txt` |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags AddressFilter
```

Base command with cli arguments:
```shell
DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> L1_SECURITY_COUNCIL=<address> npx hardhat deploy --network sepolia --tags AddressFilter
```

(make sure to replace `<key>` `<address>` with actual values)
