import { IsBoolean, IsDate, IsDecimal, IsEnum, IsNumber, IsString } from "class-validator";
import { Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn } from "typeorm";
import { Direction } from "@consensys/linea-sdk";
import { MessageStatus } from "../../../../core/enums";

@Entity({ name: "message" })
export class MessageEntity {
  @PrimaryGeneratedColumn()
  id: number;

  @Column()
  @IsString()
  messageSender: string;

  @Column()
  @IsString()
  destination: string;

  @Column()
  @IsString()
  fee: string;

  @Column()
  @IsString()
  value: string;

  @Column()
  @IsNumber()
  messageNonce: number;

  @Column()
  @IsString()
  calldata: string;

  @Column()
  @IsString()
  messageHash: string;

  @Column()
  @IsString()
  messageContractAddress: string;

  @Column()
  @IsNumber()
  sentBlockNumber: number;

  @Column()
  @IsEnum(Direction)
  direction: Direction;

  @Column()
  @IsEnum(MessageStatus)
  status: MessageStatus;

  @Column({ nullable: true })
  @IsDate()
  claimTxCreationDate?: Date;

  @Column({ nullable: true })
  claimTxGasLimit?: number;

  @Column({ nullable: true, type: "bigint" })
  claimTxMaxFeePerGas?: bigint;

  @Column({ nullable: true, type: "bigint" })
  claimTxMaxPriorityFeePerGas?: bigint;

  @Column({ nullable: true })
  claimTxNonce?: number;

  @Column({ nullable: true })
  @IsString()
  claimTxHash?: string;

  @Column()
  @IsNumber()
  claimNumberOfRetry: number;

  @Column({ nullable: true })
  @IsDate()
  claimLastRetriedAt?: Date;

  @Column({ nullable: true })
  @IsDecimal()
  claimGasEstimationThreshold?: number;

  @Column({ nullable: true })
  @IsNumber()
  compressedTransactionSize?: number;

  @Column({ default: false })
  @IsBoolean()
  isForSponsorship: boolean;

  @CreateDateColumn()
  public createdAt: Date;

  @UpdateDateColumn()
  public updatedAt: Date;
}
