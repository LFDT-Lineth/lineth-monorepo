export interface IDbNonceProvider {
  getMaxPendingNonce(): Promise<number | null>;
}
