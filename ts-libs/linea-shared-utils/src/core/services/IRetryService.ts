export interface IRetryService {
  /**
   * @notice Retry an asynchronous operation until success or failure.
   * @param fn An async callback returning a promise of type TReturn.
   */
  retry<TReturn>(fn: () => Promise<TReturn>, timeoutMs?: number): Promise<TReturn>;
}
