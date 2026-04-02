export interface INonceManager {
  initialize(): Promise<void>;
  acquireNonce(): Promise<number>;
  commitNonce(nonce: number): void;
  rollbackNonce(nonce: number): void;
}
