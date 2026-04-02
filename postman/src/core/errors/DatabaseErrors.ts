import { BaseError } from "./BaseError";
import { DatabaseErrorType, DatabaseRepoName } from "../enums";

export class DatabaseAccessError<T = unknown> extends BaseError {
  override name = DatabaseAccessError.name;
  public rejectedEntity?: T;

  constructor(name: DatabaseRepoName, type: DatabaseErrorType, cause: Error, rejectedEntity?: T) {
    super(`${name}: ${type} - ${cause.message}`, { cause });
    this.rejectedEntity = rejectedEntity;
  }
}
