import { MessageStatus } from "../enums";
import { Direction } from "@consensys/linea-sdk";

export type MessagesMetricsAttributes = {
  status: MessageStatus;
  direction: Direction;
};
