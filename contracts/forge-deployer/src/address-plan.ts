import { getAddress, getCreateAddress } from "ethers";

export type ChainName = "l1" | "l2";
export type StepId = "l1-rollup" | "l2-message-service" | "l1-token-bridge" | "l2-token-bridge";

export interface PlannedDeployment {
  key: string;
  contractName: string;
  nonce: number;
  expectedAddress: string;
}

export interface DeploymentStep {
  id: StepId;
  chain: ChainName;
  startingNonce: number;
  script: "l1-rollup" | "l2-message-service" | "token-bridge";
  deployments: PlannedDeployment[];
}

interface AddressPlanInput {
  l1Signer: string;
  l2Signer: string;
  l1StartingNonce: number;
  l2StartingNonce: number;
}

function assertNonce(value: number, name: string): void {
  if (!Number.isSafeInteger(value) || value < 0) throw new Error(`${name} must be a non-negative safe integer`);
}

function planStep(
  id: StepId,
  chain: ChainName,
  signer: string,
  startingNonce: number,
  script: DeploymentStep["script"],
  contracts: Array<[suffix: string, contractName: string]>,
): DeploymentStep {
  return {
    id,
    chain,
    startingNonce,
    script,
    deployments: contracts.map(([suffix, contractName], index) => ({
      key: `${id}.${suffix}`,
      contractName,
      nonce: startingNonce + index,
      expectedAddress: getCreateAddress({ from: signer, nonce: startingNonce + index }),
    })),
  };
}

export function buildAddressPlan(input: AddressPlanInput): DeploymentStep[] {
  assertNonce(input.l1StartingNonce, "L1 starting nonce");
  assertNonce(input.l2StartingNonce, "L2 starting nonce");
  const l1Signer = getAddress(input.l1Signer);
  const l2Signer = getAddress(input.l2Signer);

  const l1Rollup = planStep("l1-rollup", "l1", l1Signer, input.l1StartingNonce, "l1-rollup", [
    ["verifier", "IntegrationTestTrueVerifier"],
    ["implementation", "LinethRollupV8Implementation"],
    ["proxy-admin", "ProxyAdmin"],
    ["address-filter", "AddressFilter"],
    ["proxy", "LinethRollupV8"],
  ]);
  const l2MessageService = planStep("l2-message-service", "l2", l2Signer, input.l2StartingNonce, "l2-message-service", [
    ["implementation", "L2MessageServiceImplementation"],
    ["proxy-admin", "ProxyAdmin"],
    ["proxy", "L2MessageService"],
  ]);
  const l1TokenBridge = planStep(
    "l1-token-bridge",
    "l1",
    l1Signer,
    input.l1StartingNonce + l1Rollup.deployments.length,
    "token-bridge",
    [
      ["bridged-token", "BridgedToken"],
      ["implementation", "tokenBridgeContractImplementation"],
      ["proxy-admin", "ProxyAdmin"],
      ["beacon", "UpgradeableBeacon"],
      ["proxy", "TokenBridge"],
    ],
  );
  const l2TokenBridge = planStep(
    "l2-token-bridge",
    "l2",
    l2Signer,
    input.l2StartingNonce + l2MessageService.deployments.length,
    "token-bridge",
    [
      ["bridged-token", "BridgedToken"],
      ["implementation", "tokenBridgeContractImplementation"],
      ["proxy-admin", "ProxyAdmin"],
      ["beacon", "UpgradeableBeacon"],
      ["proxy", "TokenBridge"],
    ],
  );

  return [l1Rollup, l2MessageService, l1TokenBridge, l2TokenBridge];
}
