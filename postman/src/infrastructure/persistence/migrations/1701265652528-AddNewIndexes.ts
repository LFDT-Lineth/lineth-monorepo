import { MigrationInterface, QueryRunner, TableIndex } from "typeorm";

export class AddNewIndexes1701265652528 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.createIndices("message", [
      new TableIndex({
        columnNames: ["direction"],
        name: "direction_index",
      }),
      new TableIndex({
        columnNames: ["claim_tx_nonce"],
        name: "claim_tx_nonce_index",
      }),
      new TableIndex({
        columnNames: ["created_at"],
        name: "created_at_index",
      }),
      new TableIndex({
        columnNames: ["message_contract_address"],
        name: "message_contract_address_index",
      }),
      new TableIndex({
        columnNames: ["status"],
        name: "status_index",
      }),
    ]);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropIndices("message", [
      new TableIndex({
        columnNames: ["direction"],
        name: "direction_index",
      }),
      new TableIndex({
        columnNames: ["claim_tx_nonce"],
        name: "claim_tx_nonce_index",
      }),
      new TableIndex({
        columnNames: ["created_at"],
        name: "created_at_index",
      }),
      new TableIndex({
        columnNames: ["message_contract_address"],
        name: "message_contract_address_index",
      }),
      new TableIndex({
        columnNames: ["status"],
        name: "status_index",
      }),
    ]);
  }
}
