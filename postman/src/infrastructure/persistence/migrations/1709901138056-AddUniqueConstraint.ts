import { MigrationInterface, QueryRunner, TableUnique } from "typeorm";

export class AddUniqueConstraint1709901138056 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    const messageUniqueConstraint = new TableUnique({
      columnNames: ["message_hash", "direction"],
    });
    await queryRunner.createUniqueConstraint("message", messageUniqueConstraint);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    const messageUniqueConstraint = new TableUnique({
      columnNames: ["message_hash", "direction"],
    });
    await queryRunner.dropUniqueConstraint("message", messageUniqueConstraint);
  }
}
