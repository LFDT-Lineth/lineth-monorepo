import { Message } from "../types";
import { BaseError } from "./BaseError";

export class FeeEstimationError extends BaseError {
  override name = FeeEstimationError.name;
}

export class GasEstimationError<T extends Message> extends BaseError {
  override name = GasEstimationError.name;
  public rejectedMessage?: T;

  constructor(message: string, rejectedMessage?: T) {
    super(message);
    this.rejectedMessage = rejectedMessage;
  }
}
