# AddressFilter

[← Back to index](../README.md)

<br />

Deploys the AddressFilter contract used to filter addresses for forced transactions. The contract is initialized with the L1 Security Council as admin and a set of filtered addresses (in addition to default precompile addresses).

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| L1_SECURITY_COUNCIL | true | address | L1 Security Council address (contract admin) |
| ADDRESS_FILTER_FILTERED_ADDRESSES | true | address | Comma-separated list of addresses to filter (added on top of default precompiles) |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags AddressFilter
```

Base command with cli arguments:
```shell
DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> L1_SECURITY_COUNCIL=<address> ADDRESS_FILTER_FILTERED_ADDRESSES=<address1>,<address2> npx hardhat deploy --network sepolia --tags AddressFilter
```

(make sure to replace `<key>` `<address>` with actual values)
