import { Eip1193Provider, Signer, Wallet } from "ethers";
import {
  LineaRollupClient,
  EthersLineaRollupLogClient,
  L1ClaimingService,
  LineaRollupMessageRetriever,
  MerkleTreeService,
} from "./clients/ethereum";
import {
  L2MessageServiceClient,
  EthersL2MessageServiceLogClient,
  L2MessageServiceMessageRetriever,
} from "./clients/linea";
import { DefaultGasProvider, GasProvider } from "./clients/gas";
import { Provider, LineaProvider, BrowserProvider, LineaBrowserProvider } from "./clients/providers";
import {
  DEFAULT_ENABLE_LINEA_ESTIMATE_GAS,
  DEFAULT_ENFORCE_MAX_GAS_FEE,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_L2_MESSAGE_TREE_DEPTH,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
} from "./core/constants";
import { BaseError } from "./core/errors";
import { L1FeeEstimatorOptions, L2FeeEstimatorOptions, LineaSDKOptions, Network, SDKMode } from "./core/types";
import { NETWORKS } from "./core/constants";
import { isString } from "./core/utils";
import { Direction } from "./core/enums";

export class LineaSDK {
  private network: Network;
  private l1Signer?: Signer;
  private l2Signer?: Signer;
  private l1Provider: Provider | BrowserProvider;
  private l2Provider: LineaProvider | LineaBrowserProvider;
  private l1FeeEstimatorOptions: Required<L1FeeEstimatorOptions>;
  private l2FeeEstimatorOptions: Required<L2FeeEstimatorOptions>;
  private mode: SDKMode;
  private l2MessageTreeDepth: number;

  /**
   * Initializes a new instance of the LineaSDK with the specified options.
   *
   * @param {LineaSDKOptions} options - Configuration options for the SDK, including network details, operational mode, and optional settings for L2 message tree depth and fee estimation.
   */
  constructor(options: LineaSDKOptions) {
    const {
      network,
      mode,
      l2MessageTreeDepth = DEFAULT_L2_MESSAGE_TREE_DEPTH,
      l1RpcUrlOrProvider,
      l2RpcUrlOrProvider,
      l1FeeEstimatorOptions = {},
      l2FeeEstimatorOptions = {},
    } = options;

    this.network = network;
    this.mode = mode;
    this.l2MessageTreeDepth = l2MessageTreeDepth;

    this.l1Provider = this.getL1Provider(l1RpcUrlOrProvider);
    this.l2Provider = this.getL2Provider(l2RpcUrlOrProvider);

    if (mode === "read-write") {
      const { l1SignerPrivateKeyOrWallet, l2SignerPrivateKeyOrWallet } = options;

      if (!l1SignerPrivateKeyOrWallet || !l2SignerPrivateKeyOrWallet) {
        throw new BaseError("You need to provide both L1 and L2 signer private keys or wallets.");
      }

      this.l1Signer = this.getWallet(l1SignerPrivateKeyOrWallet).connect(this.l1Provider);
      this.l2Signer = this.getWallet(l2SignerPrivateKeyOrWallet).connect(this.l2Provider);
    }

    const { maxFeePerGasCap, gasFeeEstimationPercentile, enforceMaxGasFee } = l1FeeEstimatorOptions;
    const {
      maxFeePerGasCap: l2MaxFeePerGasCap,
      gasFeeEstimationPercentile: l2GasFeeEstimationPercentile,
      enableLineaEstimateGas: l2EnableLineaEstimateGas,
      enforceMaxGasFee: l2EnforceMaxGasFee,
    } = l2FeeEstimatorOptions;

    this.l1FeeEstimatorOptions = {
      maxFeePerGasCap: maxFeePerGasCap || DEFAULT_MAX_FEE_PER_GAS_CAP,
      gasFeeEstimationPercentile: gasFeeEstimationPercentile || DEFAULT_GAS_ESTIMATION_PERCENTILE,
      enforceMaxGasFee: enforceMaxGasFee || DEFAULT_ENFORCE_MAX_GAS_FEE,
    };

    this.l2FeeEstimatorOptions = {
      maxFeePerGasCap: l2MaxFeePerGasCap || DEFAULT_MAX_FEE_PER_GAS_CAP,
      gasFeeEstimationPercentile: l2GasFeeEstimationPercentile || DEFAULT_GAS_ESTIMATION_PERCENTILE,
      enforceMaxGasFee: l2EnforceMaxGasFee || DEFAULT_ENFORCE_MAX_GAS_FEE,
      enableLineaEstimateGas: l2EnableLineaEstimateGas || DEFAULT_ENABLE_LINEA_ESTIMATE_GAS,
    };
  }

  /**
   * Gets the L1 provider instance based on the provided RPC URL or provider.
   * @param {string | Eip1193Provider} l1RpcUrlOrProvider - The L1 RPC URL or EIP-1193 provider
   * @returns {Provider | BrowserProvider} The configured L1 provider
   * @throws {BaseError} If the provided argument is invalid
   */
  public getL1Provider(l1RpcUrlOrProvider: string | Eip1193Provider): Provider | BrowserProvider {
    if (isString(l1RpcUrlOrProvider)) {
      return new Provider(l1RpcUrlOrProvider);
    }

    if (this.isEip1193Provider(l1RpcUrlOrProvider)) {
      return new BrowserProvider(l1RpcUrlOrProvider);
    }

    throw new BaseError("Invalid argument: l1RpcUrlOrProvider must be a string or Eip1193Provider");
  }

  /**
   * Gets the L1 signer instance.
   * @returns {Signer} The L1 signer
   * @throws {BaseError} If the signer is not available in read-only mode
   */
  public getL1Signer(): Signer {
    if (!this.l1Signer) {
      throw new BaseError("L1 signer is not available in read-only mode.");
    }
    return this.l1Signer;
  }

  /**
   * Gets the L2 signer instance.
   * @returns {Signer} The L2 signer
   * @throws {BaseError} If the signer is not available in read-only mode
   */
  public getL2Signer(): Signer {
    if (!this.l2Signer) {
      throw new BaseError("L2 signer is not available in read-only mode.");
    }
    return this.l2Signer;
  }

  /**
   * Gets the L1 gas provider configured with the SDK's fee estimation options.
   * @returns {DefaultGasProvider} The configured L1 gas provider
   */
  public getL1GasProvider(): DefaultGasProvider {
    return new DefaultGasProvider(this.l1Provider, {
      maxFeePerGasCap: this.l1FeeEstimatorOptions.maxFeePerGasCap,
      gasEstimationPercentile: this.l1FeeEstimatorOptions.gasFeeEstimationPercentile,
      enforceMaxGasFee: this.l1FeeEstimatorOptions.enforceMaxGasFee,
    });
  }

  /**
   * Gets the L2 gas provider configured with the SDK's fee estimation options.
   * @returns {GasProvider} The configured L2 gas provider
   */
  public getL2GasProvider(): GasProvider {
    return new GasProvider(this.l2Provider, {
      maxFeePerGasCap: this.l2FeeEstimatorOptions.maxFeePerGasCap,
      enforceMaxGasFee: this.l2FeeEstimatorOptions.enforceMaxGasFee,
      gasEstimationPercentile: this.l2FeeEstimatorOptions.gasFeeEstimationPercentile,
      direction: Direction.L1_TO_L2,
      enableLineaEstimateGas: this.l2FeeEstimatorOptions.enableLineaEstimateGas,
    });
  }

  /**
   * Gets the L2 provider instance based on the provided RPC URL or provider.
   * @param {string | Eip1193Provider} l2RpcUrlOrProvider - The L2 RPC URL or EIP-1193 provider
   * @returns {LineaProvider | LineaBrowserProvider} The configured L2 provider
   * @throws {Error} If the provided argument is invalid
   */
  public getL2Provider(l2RpcUrlOrProvider: string | Eip1193Provider): LineaProvider | LineaBrowserProvider {
    if (isString(l2RpcUrlOrProvider)) {
      return new LineaProvider(l2RpcUrlOrProvider);
    }

    if (this.isEip1193Provider(l2RpcUrlOrProvider)) {
      return new LineaBrowserProvider(l2RpcUrlOrProvider);
    }

    throw new Error("Invalid argument: l2RpcUrlOrProvider must be a string or Eip1193Provider");
  }

  /**
   * Creates an instance of the `EthersLineaRollupLogClient` for interacting with L1 contract event logs.
   *
   * @param {string} [localL1ContractAddress] - Optional custom L1 contract address. Required if the network is set to 'custom'.
   * @returns {EthersLineaRollupLogClient} An instance of the L1 message service log client.
   */
  public getL1ContractEventLogClient(localL1ContractAddress?: string): EthersLineaRollupLogClient {
    const l1ContractAddress = this.getContractAddress("l1", localL1ContractAddress);
    return new EthersLineaRollupLogClient(this.l1Provider, l1ContractAddress);
  }

  /**
   * Creates an instance of the `EthersL2MessageServiceLogClient` for interacting with L2 contract event logs.
   *
   * @param {string} [localL2ContractAddress] - Optional custom L2 contract address. Required if the network is set to 'custom'.
   * @returns {EthersL2MessageServiceLogClient} An instance of the L2 message service log client.
   */
  public getL2ContractEventLogClient(localL2ContractAddress?: string): EthersL2MessageServiceLogClient {
    const l2ContractAddress = this.getContractAddress("l2", localL2ContractAddress);
    return new EthersL2MessageServiceLogClient(this.l2Provider, l2ContractAddress);
  }

  /**
   * Retrieves an instance of the `LineaRollupClient` for interacting with the L1 contract.
   *
   * @param {string} [localL1ContractAddress] - Optional custom L1 contract address. Required if the network is set to 'custom'.
   * @param {string} [localL2ContractAddress] - Optional custom L2 contract address. Required if the network is set to 'custom'.
   * @returns {LineaRollupClient} An instance of the `LineaRollupClient` configured for the specified L1 contract.
   */
  public getL1Contract(localL1ContractAddress?: string, localL2ContractAddress?: string): LineaRollupClient {
    const l1ContractAddress = this.getContractAddress("l1", localL1ContractAddress);
    const l2ContractAddress = this.getContractAddress("l2", localL2ContractAddress);

    const lineaRollupLogClient = new EthersLineaRollupLogClient(this.l1Provider, l1ContractAddress);
    const l2MessageServiceLogClient = this.getL2ContractEventLogClient(l2ContractAddress);

    return new LineaRollupClient(
      this.l1Provider,
      l1ContractAddress,
      lineaRollupLogClient,
      l2MessageServiceLogClient,
      this.getL1GasProvider(),
      new LineaRollupMessageRetriever(this.l1Provider, lineaRollupLogClient, l1ContractAddress),
      new MerkleTreeService(
        this.l1Provider,
        l1ContractAddress,
        lineaRollupLogClient,
        l2MessageServiceLogClient,
        this.l2MessageTreeDepth,
      ),
      this.mode,
      this.l1Signer,
    );
  }

  /**
   * Retrieves an instance of the `L2MessageServiceClient` for interacting with the L2 contract.
   *
   * @param {string} [localContractAddress] - Optional custom L2 contract address. Required if the network is set to 'custom'.
   * @returns {L2MessageServiceClient} An instance of the `L2MessageServiceClient` configured for the specified L2 contract.
   */
  public getL2Contract(localContractAddress?: string): L2MessageServiceClient {
    const l2ContractAddress = this.getContractAddress("l2", localContractAddress);

    const l2MessageServiceContract = new L2MessageServiceClient(
      this.l2Provider,
      l2ContractAddress,
      new L2MessageServiceMessageRetriever(
        this.l2Provider,
        this.getL2ContractEventLogClient(l2ContractAddress),
        l2ContractAddress,
      ),
      this.getL2GasProvider(),
      this.mode,
      this.l2Signer,
    );

    return l2MessageServiceContract;
  }

  /**
   * Creates an instance of the `L1ClaimingService` for managing message claiming on L1.
   *
   * @param {string} [localL1ContractAddress] - Optional custom L1 contract address. Required if the network is set to 'custom'.
   * @param {string} [localL2ContractAddress] - Optional custom L2 contract address. Required if the network is set to 'custom'.
   * @returns {L1ClaimingService} An instance of the `L1ClaimingService` configured for the specified contract addresses.
   */
  public getL1ClaimingService(localL1ContractAddress?: string, localL2ContractAddress?: string): L1ClaimingService {
    return new L1ClaimingService(
      this.getL1Contract(localL1ContractAddress, localL2ContractAddress),
      this.getL2Contract(localL2ContractAddress),
      this.getL2ContractEventLogClient(localL2ContractAddress),
      this.network,
    );
  }

  /**
   * Retrieves the contract address for the specified contract type.
   * @param contractType The type of contract to retrieve the address for.
   * @param localContractAddress Optional custom contract address.
   * @returns The contract address for the specified contract type.
   */
  private getContractAddress(contractType: "l1" | "l2", localContractAddress?: string): string {
    if (this.network === "custom") {
      if (!localContractAddress) {
        throw new BaseError(`You need to provide a ${contractType.toUpperCase()} contract address.`);
      }
      return localContractAddress;
    } else {
      const contractAddress = NETWORKS[this.network][`${contractType}ContractAddress`];
      if (!contractAddress) {
        throw new BaseError(`Contract address for ${contractType.toUpperCase()} not found in network ${this.network}.`);
      }
      return contractAddress;
    }
  }

  /**
   * Creates a wallet instance from a private key or existing wallet.
   * @param {string | Wallet} privateKeyOrWallet - The private key or wallet instance
   * @returns {Signer} The configured signer
   * @throws {BaseError} If the private key is invalid
   * @private
   */
  private getWallet(privateKeyOrWallet: string | Wallet): Signer {
    try {
      return privateKeyOrWallet instanceof Wallet ? privateKeyOrWallet : new Wallet(privateKeyOrWallet);
    } catch (e) {
      if (e instanceof Error && e.message.includes("invalid private key")) {
        throw new BaseError("Something went wrong when trying to generate Wallet. Please check your private key.");
      }
      throw e;
    }
  }

  /**
   * Type guard to check if an object implements the EIP-1193 provider interface.
   * @param {any} obj - The object to check
   * @returns {boolean} True if the object is an EIP-1193 provider
   * @private
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private isEip1193Provider(obj: any): obj is Eip1193Provider {
    return obj && typeof obj.request === "function";
  }
}
