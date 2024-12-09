export interface IMessageAnchoringProcessor {
  process(): Promise<void>;
}

export type MessageAnchoringProcessorConfig = {
  maxFetchMessagesFromDb: number;
  originContractAddress: string;
};
