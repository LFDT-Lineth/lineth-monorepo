/**
 * Contract Integrity Verifier - Tools
 *
 * Framework-agnostic tools that accept a CryptoAdapter for crypto operations.
 *
 * @packageDocumentation
 */

export {
  // Schema generator
  generateSchema,
  parseSoliditySource,
  mergeSchemas,
  calculateErc7201BaseSlot,
  // Types
  type Schema,
  type StructDef,
  type FieldDef,
  type SchemaGeneratorOptions,
  type ParseResult,
} from "./generate-schema";
