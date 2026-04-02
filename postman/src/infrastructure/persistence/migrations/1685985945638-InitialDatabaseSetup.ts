import { MigrationInterface, QueryRunner, Table, TableIndex } from "typeorm";

export class InitialDatabaseSetup1685985945638 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.createTable(
      new Table({
        name: "message",
        columns: [
          {
            name: "id",
            type: "integer",
            isPrimary: true,
            isGenerated: true,
            generationStrategy: "increment",
          },
          {
            name: "message_sender",
            type: "varchar",
          },
          {
            name: "destination",
            type: "varchar",
          },
          {
            name: "fee",
            type: "varchar",
          },
          {
            name: "value",
            type: "varchar",
          },
          {
            name: "message_nonce",
            type: "integer",
          },
          {
            name: "calldata",
            type: "varchar",
          },
          {
            name: "message_hash",
            type: "varchar",
            isUnique: true,
          },
          {
            name: "message_contract_address",
            type: "varchar",
          },
          {
            name: "sent_block_number",
            type: "integer",
          },
          {
            name: "direction",
            type: "enum",
            enum: ["L1_TO_L2", "L2_TO_L1"],
            enumName: "directionEnum",
          },
          {
            name: "status",
            type: "enum",
            enum: ["SENT", "ANCHORED", "PENDING", "CLAIMED_SUCCESS", "CLAIMED_REVERTED", "NON_EXECUTABLE"],
            enumName: "statusEnum",
          },
          {
            name: "claim_tx_creation_date",
            type: "timestamp with time zone",
            isNullable: true,
          },
          {
            name: "claim_tx_gas_limit",
            type: "integer",
            isNullable: true,
          },
          {
            name: "claim_tx_max_fee_per_gas",
            type: "bigint",
            isNullable: true,
          },
          {
            name: "claim_tx_max_priority_fee_per_gas",
            type: "bigint",
            isNullable: true,
          },
          {
            name: "claim_tx_nonce",
            type: "integer",
            isNullable: true,
          },
          {
            name: "claim_tx_hash",
            type: "varchar",
            isNullable: true,
          },
          {
            name: "created_at",
            type: "timestamp with time zone",
            default: "now()",
          },
          {
            name: "updated_at",
            type: "timestamp with time zone",
            default: "now()",
          },
        ],
      }),
      true,
    );

    const messageHashIndex = new TableIndex({
      columnNames: ["message_hash"],
      name: "message_hash_index",
    });

    const transactionHashIndex = new TableIndex({
      columnNames: ["claim_tx_hash"],
      name: "claim_tx__hash_index",
    });

    await queryRunner.createIndex("message", messageHashIndex);
    await queryRunner.createIndex("message", transactionHashIndex);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropTable("message");
    await queryRunner.dropIndices("message", [
      new TableIndex({
        columnNames: ["message_hash"],
        name: "message_hash_index",
      }),
      new TableIndex({
        columnNames: ["claim_tx_hash"],
        name: "claim_tx__hash_index",
      }),
    ]);
  }
}
