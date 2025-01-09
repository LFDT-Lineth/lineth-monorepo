# Smart Contracts

Contains Ethereum smart contract code for the Linea Rollup and Message Service.

## LineaRollup (L1MessageService)
The Linea Rollup, which contains the L1MessageService, is the smart contract that is responsible for:

- Submitting messages to be sent to Linea (L2) for later claiming.
- Anchoring of L2 message Merkle roots to allow later claiming.
- Claiming of messages sent from L2 to Ethereum mainnet (L1).
- Submission of L2 compressed data using EIP-4844 blobs or via calldata.
- Finalization of L2 state on L1 using a Zero Knowledge Proof.

## L2MessageService
The L2MessageService is the L2 smart contract that is responsible for:

- Submitting messages to be sent to L1 for later claiming.
- Anchoring of L1 to L2 Message hashes for later claiming.
- Claiming of messages sent from L1 to L2.

## Linea Canonical Token Bridge

The Canonical Token Bridge (TokenBridge) is a canonical ERC20 token brige between Ethereum and Linea networks.

The TokenBridge utilises the L1MessageService and the L2MessageService for the transmission of messages between each layer's TokenBridge.

Documentation: [Token Bridge](./docs/linea-token-bridge.md)

# Style Guide
Please see the [Smart Contract Style Guide](./docs/contract-style-guide.md) for in depth smart contract layout and styling.

# Audit reports
Please see [Audits](../docs/audits.md#linea-rollup-l2messageservice-and-tokenbridge-smart-contract-audits) for a historical list of all the smart contract audits.

# Development & Testing

Please see [Testing guidelines](./test/README.md) for in depth testing layout and styling.

This project uses the following libraries
- [PNPM](https://pnpm.io/) as the Package Manager
- [Ethers](https://github.com/ethers-io/ethers.js/) as Ethereum library
- [Hardhat](https://hardhat.org/getting-started/) as development environment
- [Chai](https://www.chaijs.com/) for assertions
- [GoLang](https://go.dev/) for the compilation of code to autogenerate data for L2 data and proofs (not strictly required)
- [Docker](https://www.docker.com/) for the local stack to run in
- [Foundry](https://book.getfoundry.sh/getting-started/installation) for Hardhat to run with `hardhat-foundry` plugin

If you already have an understanding of the tech stack, use our [Get Started](../docs/get-started.md) guide.

To run the tests:

## Testing without coverage

```bash
cd contracts # from the root folder
pnpm install
npx hardhat test
```

## Testing with coverage

```bash
cd contracts # from the root folder
pnpm install

npx hardhat coverage
```
## Deploying the contracts to the local stack
Prerequisites: 
- Be sure Docker is running.

Some caveats:
- The L2 chain will not produce empty blocks if there are no transactions, so it would be useful to execute a script to keep the chain "moving". 
  - The following script can be run with the expectation the local stack is running: [generateL2Traffic.ts](../e2e/src/common/generateL2Traffic.ts)
  - To execute it run the following from the `e2e` folder: `npx ts-node src/common/generateL2Traffic.ts`
- For blob submission and finalization, there needs to be sufficient blocks to trigger it. Keeping the chain moving on L2 is vital for this to take place.

From the root of the repository:

Please read the MakeFile: [MakeFile](../Makefile)

```
# This will deploy all the relevant services and smart contracts

make fresh-start-all-traces-v2
```

### To deploy all the contracts

If the stack is *already running*, to redeploy the contracts, the following commands can be used:

Note: The addresses change per deployment due to nonce increments, so be sure to validate the correct ones are being used.

**NB:** The end to end tests run against a fresh stack and deploy with predetermined addresses.

If there is a need to get predetermined addresses for contract deployments, the following script can be used [precomputeDeployedAddresses.ts](./scripts/operational/precomputeDeployedAddress.ts).

This can be used by altering the values in the script file and running the script (from the `/contracts` folder) with: `npx ts-node scripts/operational/precomputeDeployedAddress.ts`

*Note the following nonce values for a fresh stack deploy:*

The LineaRollup deploy uses nonce 3 as the following are deployed beforehand:
- The verifier contract
- The implementation LineaRollup.sol contract
- The proxy admin contract

The L2MessageService deploy uses nonce 2 as the following are deployed beforehand:
- The implementation L2MessageService.sol contract
- The proxy admin contract


**Deploying the L1 contracts**
```
# This will deploy the Linea Rollup that is currently deployed on Mainnet - the current version is the LineaRollupV5.
# Some end to end tests will test future upgrades to validate the stack remains functional.

# Note: By default a test/placeholder verifier contract is used `IntegrationTestTrueVerifier` if you wish to use a proper verifier, adjust the
# PLONKVERIFIER_NAME=IntegrationTestTrueVerifier in the make command to be something like PLONKVERIFIER_NAME=PlonkVerifierForDataAggregation .

# Be sure to check the parameter values in the Makefile before executing the command.

# Deploy v5
make deploy-linea-rollup-v5 

# Or deploy v6
make deploy-linea-rollup-v6

make deploy-token-bridge-l1
```

**Deploying the L2 contracts**
```
# This will deploy the current L2 Message Service.
# Some end to end tests will test future upgrades to validate the stack remains functional.

make deploy-l2messageservice

make deploy-token-bridge-l2
```

**Deploying L1 and L2 together**
```
make deploy-contracts
```

The above command will trigger the following commands to deploy:

- deploy-linea-rollup-v5 
- deploy-token-bridge-l1 
- deploy-l1-test-erc20 
- deploy-l2messageservice 
- deploy-token-bridge-l2 
- deploy-l2-test-erc20

Note: the deploy-l1-test-erc20 and deploy-l2-test-erc20 commands are executed for use in the end to end tests.

## Installation and testing

To run the solution's tests, coverage and gas reporting, be sure to install pnpm and then
```
# Install all the dependencies

pnpm install

pnpm run test

pnpm run test:reportgas

pnpm run coverage
```
