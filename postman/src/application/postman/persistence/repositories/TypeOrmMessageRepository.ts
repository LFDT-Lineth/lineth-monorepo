/* eslint-disable @typescript-eslint/no-explicit-any */
import { Brackets, DataSource, Repository } from "typeorm";
import { ContractTransactionResponse } from "ethers";
import { Direction } from "@consensys/linea-sdk";
import { Message } from "../../../../core/entities/Message";
import { mapMessageEntityToMessage, mapMessageToMessageEntity } from "../mappers/messageMappers";
import { DatabaseErrorType, DatabaseRepoName, MessageStatus } from "../../../../core/enums";
import { DatabaseAccessError } from "../../../../core/errors";
import { MessageEntity } from "../entities/Message.entity";
import { subtractSeconds } from "../../../../core/utils/shared";
import { IMessageRepository } from "../../../../core/persistence/IMessageRepository";

export class TypeOrmMessageRepository<TransactionResponse extends ContractTransactionResponse>
  extends Repository<MessageEntity>
  implements IMessageRepository<TransactionResponse>
{
  constructor(readonly dataSource: DataSource) {
    super(MessageEntity, dataSource.createEntityManager());
  }

  async findByMessageHash(message: Message, direction: Direction): Promise<Message | null> {
    try {
      const messageInDb = await this.findOneBy({
        messageHash: message.messageHash,
        direction,
      });
      if (!messageInDb) {
        return null;
      }
      return mapMessageEntityToMessage(messageInDb);
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err, message);
    }
  }

  async insertMessage(message: Message): Promise<void> {
    try {
      const messageInDb = await this.findOneBy({
        messageHash: message.messageHash,
        direction: message.direction,
      });
      if (!messageInDb) {
        await this.manager.save(MessageEntity, mapMessageToMessageEntity(message));
      }
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Insert, err, message);
    }
  }

  async updateMessage(message: Message): Promise<void> {
    try {
      const messageInDb = await this.findOneBy({
        messageHash: message.messageHash,
        direction: message.direction,
      });
      if (messageInDb) {
        await this.manager.save(MessageEntity, mapMessageToMessageEntity(message));
      }
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Update, err, message);
    }
  }

  async updateMessageByTransactionHash(transactionHash: string, direction: Direction, message: Message): Promise<void> {
    try {
      const messageInDb = await this.findOneBy({
        claimTxHash: transactionHash,
        direction,
      });
      if (messageInDb) {
        await this.manager.save(MessageEntity, mapMessageToMessageEntity(message));
      }
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Update, err);
    }
  }

  async saveMessages(messages: Message[]): Promise<void> {
    try {
      await this.manager.save(MessageEntity, messages.map(mapMessageToMessageEntity));
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Insert, err);
    }
  }

  async deleteMessages(msBeforeNowToDelete: number): Promise<number> {
    try {
      const d = new Date();
      d.setTime(d.getTime() - msBeforeNowToDelete);
      const formattedDateStr = d.toISOString().replace("T", " ").replace("Z", "");
      const deleteResult = await this.createQueryBuilder("message")
        .delete()
        .where("message.status IN(:...statuses)", {
          statuses: [
            MessageStatus.CLAIMED_SUCCESS,
            MessageStatus.CLAIMED_REVERTED,
            MessageStatus.EXCLUDED,
            MessageStatus.ZERO_FEE,
          ],
        })
        .andWhere("message.updated_at < :updated_before", { updated_before: formattedDateStr })
        .execute();
      return deleteResult.affected ?? 0;
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Delete, err);
    }
  }

  async getFirstMessageToClaimOnL1(
    direction: Direction,
    contractAddress: string,
    currentGasPrice: bigint,
    gasEstimationMargin: number,
    maxRetry: number,
    retryDelay: number,
  ): Promise<Message | null> {
    try {
      const message = await this.createQueryBuilder("message")
        .where("message.direction = :direction", { direction })
        .andWhere("message.messageContractAddress = :contractAddress", { contractAddress })
        .andWhere("message.status IN(:...statuses)", {
          statuses: [MessageStatus.ANCHORED, MessageStatus.FEE_UNDERPRICED],
        })
        .andWhere("message.claimNumberOfRetry < :maxRetry", { maxRetry })
        .andWhere(
          new Brackets((qb) => {
            qb.where("message.claimLastRetriedAt IS NULL").orWhere("message.claimLastRetriedAt < :lastRetriedDate", {
              lastRetriedDate: subtractSeconds(new Date(), retryDelay).toISOString(),
            });
          }),
        )
        .andWhere(
          new Brackets((qb) => {
            qb.where("message.claimGasEstimationThreshold > :threshold", {
              threshold: parseFloat(currentGasPrice.toString()) * gasEstimationMargin,
            }).orWhere("message.claimGasEstimationThreshold IS NULL");
          }),
        )
        .orderBy("CAST(message.status as CHAR)", "ASC")
        .addOrderBy("message.claimGasEstimationThreshold", "DESC")
        .addOrderBy("message.sentBlockNumber", "ASC")
        .getOne();

      return message ? mapMessageEntityToMessage(message) : null;
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async getFirstMessageToClaimOnL2(
    direction: Direction,
    contractAddress: string,
    messageStatuses: MessageStatus[],
    maxRetry: number,
    retryDelay: number,
    feeEstimationOptions: {
      minimumMargin: number;
      extraDataVariableCost: number;
      extraDataFixedCost: number;
    },
  ): Promise<Message | null> {
    try {
      const message = await this.createQueryBuilder("message")
        .where("message.direction = :direction", { direction })
        .andWhere("message.messageContractAddress = :contractAddress", { contractAddress })
        .andWhere("message.status IN(:...statuses)", {
          statuses: messageStatuses,
        })
        .andWhere("message.claimNumberOfRetry < :maxRetry", { maxRetry })
        .andWhere(
          new Brackets((qb) => {
            qb.where("message.claimLastRetriedAt IS NULL").orWhere("message.claimLastRetriedAt < :lastRetriedDate", {
              lastRetriedDate: subtractSeconds(new Date(), retryDelay).toISOString(),
            });
          }),
        )
        .andWhere(
          "CAST(message.fee AS numeric) > :minimumMargin * ((:extraDataVariableCost * message.compressedTransactionSize) / message.claimTxGasLimit + :extraDataFixedCost) * message.claimTxGasLimit",
          {
            minimumMargin: feeEstimationOptions.minimumMargin,
            extraDataVariableCost: feeEstimationOptions.extraDataVariableCost,
            extraDataFixedCost: feeEstimationOptions.extraDataFixedCost,
          },
        )
        .orderBy("CAST(message.status as CHAR)", "ASC")
        .addOrderBy("CAST(message.fee AS numeric)", "DESC")
        .addOrderBy("message.sentBlockNumber", "ASC")
        .getOne();

      return message ? mapMessageEntityToMessage(message) : null;
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async getLatestMessageSent(direction: Direction, contractAddress: string): Promise<Message | null> {
    try {
      const pendingMessages = await this.find({
        where: {
          direction,
          messageContractAddress: contractAddress,
        },
        take: 1,
        order: {
          createdAt: "DESC",
        },
      });

      if (pendingMessages.length === 0) return null;

      return mapMessageEntityToMessage(pendingMessages[0]);
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async getNFirstMessagesByStatus(
    status: MessageStatus,
    direction: Direction,
    limit: number,
    contractAddress: string,
  ): Promise<Message[]> {
    try {
      const messages = await this.find({
        where: {
          direction,
          status,
          messageContractAddress: contractAddress,
        },
        take: limit,
        order: {
          sentBlockNumber: "ASC",
        },
      });
      return messages.map(mapMessageEntityToMessage);
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async getMessageSent(direction: Direction, contractAddress: string): Promise<Message | null> {
    try {
      const message = await this.findOne({
        where: {
          direction,
          status: MessageStatus.SENT,
          messageContractAddress: contractAddress,
        },
      });
      return message ? mapMessageEntityToMessage(message) : null;
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async getLastClaimTxNonce(direction: Direction): Promise<number | null> {
    try {
      const message = await this.createQueryBuilder("message")
        .select("MAX(message.claimTxNonce)", "lastTxNonce")
        .where("message.direction = :direction", { direction })
        .getRawOne();

      if (!message.lastTxNonce) {
        return null;
      }

      return message.lastTxNonce;
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async getFirstPendingMessage(direction: Direction): Promise<Message | null> {
    try {
      const message = await this.createQueryBuilder("message")
        .where("message.direction = :direction", { direction })
        .andWhere("message.status = :status", { status: MessageStatus.PENDING })
        .orderBy("message.claimTxNonce", "ASC")
        .getOne();

      return message ? mapMessageEntityToMessage(message) : null;
    } catch (err: any) {
      throw new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, err);
    }
  }

  async updateMessageWithClaimTxAtomic(
    message: Message,
    nonce: number,
    claimTxResponsePromise: Promise<ContractTransactionResponse>,
  ): Promise<void> {
    await this.manager.transaction(async (entityManager) => {
      await entityManager.update(
        MessageEntity,
        { messageHash: message.messageHash, direction: message.direction },
        {
          claimTxCreationDate: new Date(),
          claimTxNonce: nonce,
          status: MessageStatus.PENDING,
          ...(message.status === MessageStatus.FEE_UNDERPRICED
            ? { claimNumberOfRetry: message.claimNumberOfRetry + 1, claimLastRetriedAt: new Date() }
            : {}),
        },
      );

      const tx = await claimTxResponsePromise;

      await entityManager.update(
        MessageEntity,
        { messageHash: message.messageHash, direction: message.direction },
        {
          claimTxGasLimit: parseInt(tx.gasLimit.toString()),
          claimTxMaxFeePerGas: tx.maxFeePerGas ?? undefined,
          claimTxMaxPriorityFeePerGas: tx.maxPriorityFeePerGas ?? undefined,
          claimTxHash: tx.hash,
        },
      );
    });
  }
}
