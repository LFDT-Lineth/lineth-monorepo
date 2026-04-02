import { Direction } from "../enums";
import { MessageStatus } from "../enums";

export type MessagesMetricsAttributes = {
  status: MessageStatus;
  direction: Direction;
};
