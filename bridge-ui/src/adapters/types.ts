import { Address } from "viem";
import { Config } from "wagmi";

import { type HistoryActionsForCompleteTxCaching } from "@/stores/historyStore";
import {
  type AdapterModeId,
  BridgeProvider,
  type BridgeMessage,
  BridgeTransaction,
  Chain,
  ChainLayer,
  ClaimType,
  GithubTokenListToken,
  SupportedChainIds,
  Token,
} from "@/types";

export interface TransactionRequest {
  to: Address;
  data: `0x${string}`;
  value: bigint;
  chainId: SupportedChainIds;
}

export interface BridgeOptions {
  selectedMode?: AdapterModeId;
  manualClaim?: boolean;
}

export interface BridgeFees {
  protocolFee: bigint | null;
  bridgingFee: bigint;
  claimType: ClaimType;
}

export interface FeeParams {
  amount: bigint;
  token: Token;
  fromChain: Chain;
  toChain: Chain;
  address: Address;
  recipient: Address;
  wagmiConfig: Config;
  options?: BridgeOptions;
}

export interface DepositParams {
  token: Token;
  amount: bigint;
  recipient: Address;
  fromChain: Chain;
  toChain: Chain;
  fees: BridgeFees;
  options?: BridgeOptions;
}

export interface ClaimParams {
  message: BridgeMessage;
  fromChain: Chain;
  toChain: Chain;
}

export interface HistoryParams {
  historyStoreActions: HistoryActionsForCompleteTxCaching;
  address: Address;
  fromChain: Chain;
  toChain: Chain;
  tokens: Token[];
  wagmiConfig: Config;
}

export interface AdapterMode {
  readonly id: AdapterModeId;
  readonly label: string;
  readonly description: string;
  readonly logoSrc: string;
}

export type TimeUnit = "second" | "minute" | "hour";

export interface EstimatedTime {
  min: number;
  max: number;
  unit: TimeUnit;
}

export interface TransactionStep {
  id: string;
  label: string;
  tx: TransactionRequest;
}

export interface PreStepParams {
  token: Token;
  fromChain: Chain;
  amount: bigint;
  allowance: bigint | undefined;
}

export interface ReceivedAmountParams {
  amount: bigint;
  token: Token;
  fromChainLayer: ChainLayer;
  fees: BridgeFees;
}

export interface BridgeAdapter {
  readonly id: string;
  readonly name: string;
  readonly provider: BridgeProvider;
  readonly logoSrc: string;
  readonly modes?: readonly AdapterMode[];
  readonly defaultMode?: AdapterModeId;
  readonly hasAdvancedSettings?: boolean;
  /** Custom label for the destination-chain bridging fee row (defaults to "<toChain.name> fee"). */
  readonly bridgingFeeLabel?: string;

  isEnabled(): boolean;
  matchesToken(token: GithubTokenListToken): boolean;
  canHandle(token: Token, fromChain: Chain, toChain: Chain): boolean;

  getEstimatedTime?(fromChainLayer: ChainLayer, mode: AdapterModeId | undefined): EstimatedTime;

  buildDepositTx(params: DepositParams): TransactionRequest | undefined;
  buildClaimTx?(params: ClaimParams): TransactionRequest | undefined;

  /**
   * Enriches the message with data needed for claiming (e.g., Merkle proof for native L2→L1).
   * Called before buildClaimTx when the transaction is ready to claim.
   */
  prepareClaimMessage?(params: ClaimParams & { wagmiConfig: Config }): Promise<void>;

  getApprovalTarget(token: Token, fromChain: Chain): Address | undefined;
  computeReceivedAmount(params: ReceivedAmountParams): bigint;

  /**
   * Optional custom pre-steps before the bridge tx (e.g., permit, wrap).
   * The generic approval step is always auto-generated from getApprovalTarget().
   */
  getPreSteps?(params: PreStepParams): TransactionStep[];

  fetchHistory(params: HistoryParams): Promise<BridgeTransaction[]>;

  /**
   * Computes all adapter-specific fees (protocol fee, bridging fee, etc.)
   * and resolves the claim type. The hook wraps this in useQuery.
   */
  getFees?(params: FeeParams): Promise<BridgeFees>;

  /**
   * Looks up the claiming transaction hash on the destination chain
   * for a completed bridge transaction.
   */
  getClaimingTxHash?(message: BridgeMessage, toChain: Chain, wagmiConfig: Config): Promise<string | undefined>;

  /**
   * Fallback gas limit for origin chain tx estimation when user is disconnected.
   * Return undefined to attempt real estimation even when disconnected (e.g. ETH via stateOverride).
   */
  getFallbackGasLimit?(token: Token): bigint | undefined;
}
