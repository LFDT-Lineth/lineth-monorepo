# LineaSequencerUptimeFeed

[← Back to index](../README.md)

<br />

Deploys the Linea Sequencer Uptime Feed contract (Chainlink-compatible).

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_SEQUENCER_UPTIME_FEED_INITIAL_STATUS | true | uint256 | Initial feed status |
| LINEA_SEQUENCER_UPTIME_FEED_ADMIN | true | address | Admin address |
| LINEA_SEQUENCER_UPTIME_FEED_UPDATER | true | address | Updater address |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags LineaSequencerUptimeFeed
```
