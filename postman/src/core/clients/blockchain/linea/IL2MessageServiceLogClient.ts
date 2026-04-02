import { IMessageSentEventLogClient } from "../ILogClient";

export type { MessageSentEventFilters, GetMessageSentEventsParams } from "../ILogClient";

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface IL2MessageServiceLogClient extends IMessageSentEventLogClient {}
