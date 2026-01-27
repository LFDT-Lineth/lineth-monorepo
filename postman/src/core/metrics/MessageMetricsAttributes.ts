import { Direction } from "@consensys/linea-sdk";

import { MessageStatus } from "../enums";

export type MessagesMetricsAttributes = {
  status: MessageStatus;
  direction: Direction;
};
