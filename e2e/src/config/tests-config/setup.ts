import { AbstractSigner, JsonRpcProvider, Wallet } from "ethers";
import { Config, L2Config, LocalL2Config } from "./types";
import {
  BridgedToken,
  BridgedToken__factory,
  DummyContract,
  DummyContract__factory,
  L2MessageService,
  L2MessageService__factory,
  LineaRollupV6,
  LineaRollupV6__factory,
  ProxyAdmin,
  ProxyAdmin__factory,
  TestContract,
  TestContract__factory,
  TestERC20,
  TestERC20__factory,
  TokenBridge,
  TokenBridge__factory,
} from "../../typechain";
import { AccountManager } from "./accounts/account-manager";

export default class TestSetup {
  constructor(private readonly config: Config) {}

  public getL1Provider(): JsonRpcProvider {
    return new JsonRpcProvider(this.config.L1.rpcUrl.toString());
  }

  public getL2Provider(): JsonRpcProvider {
    return new JsonRpcProvider(this.config.L2.rpcUrl.toString());
  }

  public getL2SequencerProvider(): JsonRpcProvider | undefined {
    if (!this.isLocalL2Config(this.config.L2)) {
      return undefined;
    }
    return new JsonRpcProvider(this.config.L2.sequencerEndpoint.toString());
  }

  public getL2BesuNodeProvider(): JsonRpcProvider | undefined {
    if (!this.isLocalL2Config(this.config.L2)) {
      return undefined;
    }
    return new JsonRpcProvider(this.config.L2.besuNodeRpcUrl.toString());
  }

  public getL1ChainId(): number {
    return this.config.L1.chainId;
  }

  public getL2ChainId(): number {
    return this.config.L2.chainId;
  }

  public getShomeiEndpoint(): URL | undefined {
    if (!this.isLocalL2Config(this.config.L2)) {
      return undefined;
    }
    return this.config.L2.shomeiEndpoint;
  }

  public getShomeiFrontendEndpoint(): URL | undefined {
    if (!this.isLocalL2Config(this.config.L2)) {
      return undefined;
    }
    return this.config.L2.shomeiFrontendEndpoint;
  }

  public getSequencerEndpoint(): URL | undefined {
    if (!this.isLocalL2Config(this.config.L2)) {
      return undefined;
    }
    return this.config.L2.sequencerEndpoint;
  }

  public getTransactionExclusionEndpoint(): URL | undefined {
    if (!this.isLocalL2Config(this.config.L2)) {
      return undefined;
    }
    return this.config.L2.transactionExclusionEndpoint;
  }

  public getLineaRollupContract(signer?: AbstractSigner): LineaRollupV6 {
    const lineaRollup: LineaRollupV6 = LineaRollupV6__factory.connect(
      this.config.L1.lineaRollupAddress,
      this.getL1Provider(),
    );

    if (signer) {
      return lineaRollup.connect(signer);
    }

    return lineaRollup;
  }

  public getLineaRollupProxyAdminContract(signer?: AbstractSigner): ProxyAdmin {
    const proxyAdmin: ProxyAdmin = ProxyAdmin__factory.connect(
      this.config.L1.lineaRollupProxyAdminAddress,
      this.getL1Provider(),
    );

    if (signer) {
      return proxyAdmin.connect(signer);
    }

    return proxyAdmin;
  }

  public getL2MessageServiceContract(signer?: Wallet): L2MessageService {
    const l2MessageService: L2MessageService = L2MessageService__factory.connect(
      this.config.L2.l2MessageServiceAddress,
      this.getL2Provider(),
    );

    if (signer) {
      return l2MessageService.connect(signer);
    }

    return l2MessageService;
  }

  public getL1TokenBridgeContract(signer?: Wallet): TokenBridge {
    const l1TokenBridge: TokenBridge = TokenBridge__factory.connect(
      this.config.L1.tokenBridgeAddress,
      this.getL1Provider(),
    );

    if (signer) {
      return l1TokenBridge.connect(signer);
    }

    return l1TokenBridge;
  }

  public getL2TokenBridgeContract(signer?: Wallet): TokenBridge {
    const l2TokenBridge: TokenBridge = TokenBridge__factory.connect(
      this.config.L2.tokenBridgeAddress,
      this.getL2Provider(),
    );

    if (signer) {
      return l2TokenBridge.connect(signer);
    }

    return l2TokenBridge;
  }

  public getL1TokenContract(signer?: Wallet): TestERC20 {
    const l1Token: TestERC20 = TestERC20__factory.connect(this.config.L1.l1TokenAddress, this.getL1Provider());

    if (signer) {
      return l1Token.connect(signer);
    }

    return l1Token;
  }

  public getL2TokenContract(signer?: Wallet): TestERC20 {
    const l2Token: TestERC20 = TestERC20__factory.connect(this.config.L2.l2TokenAddress, this.getL2Provider());

    if (signer) {
      return l2Token.connect(signer);
    }

    return l2Token;
  }

  public getL1BridgedTokenContract(bridgedTokenAddress: string, signer?: Wallet): BridgedToken {
    const l1BridgedToken: BridgedToken = BridgedToken__factory.connect(bridgedTokenAddress, this.getL1Provider());

    if (signer) {
      return l1BridgedToken.connect(signer);
    }

    return l1BridgedToken;
  }

  public getL2BridgedTokenContract(bridgedTokenAddress: string, signer?: Wallet): BridgedToken {
    const l2BridgedToken: BridgedToken = BridgedToken__factory.connect(bridgedTokenAddress, this.getL2Provider());

    if (signer) {
      return l2BridgedToken.connect(signer);
    }

    return l2BridgedToken;
  }
  public getL1DummyContract(signer?: Wallet): DummyContract {
    const dummyContract = DummyContract__factory.connect(this.config.L1.dummyContractAddress, this.getL1Provider());

    if (signer) {
      return dummyContract.connect(signer);
    }

    return dummyContract;
  }

  public getL2DummyContract(signer?: Wallet): DummyContract {
    const dummyContract = DummyContract__factory.connect(this.config.L2.dummyContractAddress, this.getL2Provider());

    if (signer) {
      return dummyContract.connect(signer);
    }

    return dummyContract;
  }

  public getL2TestContract(signer?: Wallet): TestContract | undefined {
    if (this.config.L2.l2TestContractAddress) {
      const testContract = TestContract__factory.connect(this.config.L2.l2TestContractAddress, this.getL2Provider());

      if (signer) {
        return testContract.connect(signer);
      }

      return testContract;
    } else {
      return undefined;
    }
  }

  public getL1AccountManager(): AccountManager {
    return this.config.L1.accountManager;
  }

  public getL2AccountManager(): AccountManager {
    return this.config.L2.accountManager;
  }

  public getL1DummyContractAddress(): string {
    return this.config.L1.dummyContractAddress;
  }

  private isLocalL2Config(config: L2Config): config is LocalL2Config {
    return (
      (config as LocalL2Config).besuNodeRpcUrl !== undefined &&
      (config as LocalL2Config).sequencerEndpoint !== undefined &&
      (config as LocalL2Config).shomeiEndpoint !== undefined &&
      (config as LocalL2Config).shomeiFrontendEndpoint !== undefined &&
      (config as LocalL2Config).transactionExclusionEndpoint !== undefined
    );
  }
}
