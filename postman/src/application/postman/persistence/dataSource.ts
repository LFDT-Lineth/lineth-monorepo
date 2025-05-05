import { DataSource } from "typeorm";
import { SnakeNamingStrategy } from "typeorm-naming-strategies";
import { InitialDatabaseSetup1685985945638 } from "./migrations/1685985945638-InitialDatabaseSetup";
import { AddNewColumns1687890326970 } from "./migrations/1687890326970-AddNewColumns";
import { UpdateStatusColumn1687890694496 } from "./migrations/1687890694496-UpdateStatusColumn";
import { RemoveUniqueConstraint1689084924789 } from "./migrations/1689084924789-RemoveUniqueConstraint";
import { AddNewIndexes1701265652528 } from "./migrations/1701265652528-AddNewIndexes";
import { AddUniqueConstraint1709901138056 } from "./migrations/1709901138056-AddUniqueConstraint";
import { DBOptions } from "./config/types";
import { MessageEntity } from "./entities/Message.entity";
import { AddCompressedTxSizeColumn1718026260629 } from "./migrations/1718026260629-AddCompressedTxSizeColumn";
import { AddSponsorshipMetrics1745569276097 } from "./migrations/1745569276097-AddSponsorshipMetrics";
export class DB {
  public static create(config: DBOptions): DataSource {
    return new DataSource({
      ...config,
      entities: [MessageEntity],
      namingStrategy: new SnakeNamingStrategy(),
      migrations: [
        InitialDatabaseSetup1685985945638,
        AddNewColumns1687890326970,
        UpdateStatusColumn1687890694496,
        RemoveUniqueConstraint1689084924789,
        AddNewIndexes1701265652528,
        AddUniqueConstraint1709901138056,
        AddCompressedTxSizeColumn1718026260629,
        AddSponsorshipMetrics1745569276097,
      ],
      migrationsTableName: "migrations",
      logging: ["error"],
      migrationsRun: true,
    });
  }
}
