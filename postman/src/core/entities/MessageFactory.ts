import { Message, MessageProps } from "./Message";

export class MessageFactory {
  public static createMessage(params: MessageProps): Message {
    return new Message(params);
  }
}
