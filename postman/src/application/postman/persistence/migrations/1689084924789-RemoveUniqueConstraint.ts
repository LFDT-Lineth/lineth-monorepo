import { MigrationInterface, QueryRunner, TableUnique } from "typeorm";

export class RemoveUniqueConstraint1689084924789 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropUniqueConstraint("message", "UQ_4ae806cf878a218ad891a030ab5");
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.createUniqueConstraint("message", new TableUnique({ columnNames: ["message_hash"] }));
  }
}
