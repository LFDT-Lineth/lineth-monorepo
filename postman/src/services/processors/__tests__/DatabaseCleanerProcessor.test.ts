import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { DatabaseErrorType, DatabaseRepoName } from "../../../core/enums";
import { DatabaseAccessError } from "../../../core/errors/DatabaseErrors";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { DatabaseCleanerProcessor } from "../DatabaseCleanerProcessor";

describe("DatabaseCleanerProcessor", () => {
  let databaseCleanerProcessor: DatabaseCleanerProcessor;
  const messageRepositoryMock = mock<IMessageRepository>();
  const loggerMock = mock<ILogger>();

  beforeEach(() => {
    databaseCleanerProcessor = new DatabaseCleanerProcessor(
      messageRepositoryMock,
      { daysBeforeNowToDelete: 10 },
      loggerMock,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("databaseCleanerRoutine", () => {
    it("Should log info if deleteMessages returns successfully", async () => {
      const messageRepositorySpy = jest.spyOn(messageRepositoryMock, "deleteMessages").mockResolvedValue(10);
      const loggerInfoSpy = jest.spyOn(loggerMock, "info");

      await databaseCleanerProcessor.process();

      expect(messageRepositorySpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Database cleanup result: deleted %s rows", 10);
    });

    it("Should log error if deleteMessages returns error", async () => {
      const messageRepositorySpy = jest
        .spyOn(messageRepositoryMock, "deleteMessages")
        .mockRejectedValue(
          new DatabaseAccessError(
            DatabaseRepoName.MessageRepository,
            DatabaseErrorType.Delete,
            new Error("Error for testing"),
          ),
        );
      const loggerErrorSpy = jest.spyOn(loggerMock, "error");

      await databaseCleanerProcessor.process();

      expect(messageRepositorySpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Database cleanup failed.", {
        error: new DatabaseAccessError(
          DatabaseRepoName.MessageRepository,
          DatabaseErrorType.Delete,
          new Error("Error for testing"),
        ),
      });
    });
  });
});
