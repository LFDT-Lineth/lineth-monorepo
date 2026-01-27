import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, beforeEach } from "@jest/globals";
import { ContractTransactionResponse } from "ethers";
import { mock } from "jest-mock-extended";

import { DatabaseErrorType, DatabaseRepoName } from "../../../core/enums";
import { DatabaseAccessError } from "../../../core/errors/DatabaseErrors";
import { IMessageDBService } from "../../../core/persistence/IMessageDBService";
import { DatabaseCleaner } from "../DatabaseCleaner";

describe("TestDatabaseCleaner", () => {
  let testDatabaseCleaner: DatabaseCleaner;
  const messageRepositoryMock = mock<IMessageDBService<ContractTransactionResponse>>();
  const loggerMock = mock<ILogger>();

  beforeEach(() => {
    testDatabaseCleaner = new DatabaseCleaner(messageRepositoryMock, loggerMock);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("databaseCleanerRoutine", () => {
    it("Should log info if deleteMessages returns successfully", async () => {
      const messageRepositorySpy = jest.spyOn(messageRepositoryMock, "deleteMessages").mockResolvedValue(10);
      const loggerInfoSpy = jest.spyOn(loggerMock, "info");

      await testDatabaseCleaner.databaseCleanerRoutine(10 * 24 * 3600 * 1000); // ms for 10 days

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

      await testDatabaseCleaner.databaseCleanerRoutine(10 * 24 * 3600 * 1000); // ms for 10 days

      expect(messageRepositorySpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith(
        new DatabaseAccessError(
          DatabaseRepoName.MessageRepository,
          DatabaseErrorType.Delete,
          new Error("Error for testing"),
        ),
      );
    });
  });
});
