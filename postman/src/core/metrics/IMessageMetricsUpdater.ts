import { MessagesMetricsAttributes } from "./MessageMetricsAttributes";

export interface IMessageMetricsUpdater {
  // Method to initialize the message gauges
  initialize(): Promise<void>;
  // Method to get the current message count for given attributes
  getMessageCount(messageAttributes: MessagesMetricsAttributes): Promise<number | undefined>;
  // Method to increment the message count for given attributes and value
  incrementMessageCount(messageAttributes: MessagesMetricsAttributes, value?: number): Promise<void>;
  // Method to decrement the message count for given attributes and value
  decrementMessageCount(messageAttributes: MessagesMetricsAttributes, value?: number): Promise<void>;
}
