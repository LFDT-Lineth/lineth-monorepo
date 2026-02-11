import { type Abi, type Address, type Client, type Transport, type Chain, type Account, getContract } from "viem";

import {
  BridgedTokenAbi,
  DummyContractAbi,
  L2MessageServiceV1Abi,
  LineaRollupV6Abi,
  LineaSequencerUptimeFeedAbi,
  OpcodeTesterAbi,
  ProxyAdminAbi,
  SparseMerkleProofAbi,
  TestContractAbi,
  TestERC20Abi,
  TokenBridgeV1_1Abi,
} from "../../generated";

function createContractGetter<const TAbi extends Abi>(abi: TAbi) {
  return <
    transport extends Transport = Transport,
    chain extends Chain | undefined = Chain | undefined,
    account extends Account | undefined = Account | undefined,
  >(
    client: Client<transport, chain, account>,
    address: Address,
  ) => getContract({ abi, address, client });
}

export const getLineaRollupContract = createContractGetter(LineaRollupV6Abi);
export const getLineaRollupProxyAdminContract = createContractGetter(ProxyAdminAbi);
export const getTestERC20Contract = createContractGetter(TestERC20Abi);
export const getTokenBridgeContract = createContractGetter(TokenBridgeV1_1Abi);
export const getDummyContract = createContractGetter(DummyContractAbi);
export const getL2MessageServiceContract = createContractGetter(L2MessageServiceV1Abi);
export const getSparseMerkleProofContract = createContractGetter(SparseMerkleProofAbi);
export const getLineaSequencerUpTimeFeedContract = createContractGetter(LineaSequencerUptimeFeedAbi);
export const getOpcodeTesterContract = createContractGetter(OpcodeTesterAbi);
export const getTestContract = createContractGetter(TestContractAbi);
export const getBridgedTokenContract = createContractGetter(BridgedTokenAbi);
