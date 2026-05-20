export interface IProposalProcessor {
  processOnce(): Promise<void>;
}
