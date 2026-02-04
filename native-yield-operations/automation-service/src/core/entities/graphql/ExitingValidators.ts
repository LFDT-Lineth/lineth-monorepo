import { gql, TypedDocumentNode } from "@apollo/client";
import { ExitingValidator } from "../Validator.js";
// https://www.apollographql.com/docs/react/data/typescript#using-operation-types

/** Result shape returned by the server */
type ExitingValidatorsQuery = {
  allHeadValidators: {
    nodes: Array<ExitingValidator>;
  };
};

/** Variables you pass in */
type ExitingValidatorsQueryVariables = {
  first?: number; // optional since we set a default in the query
};

export const EXITING_VALIDATORS_QUERY: TypedDocumentNode<ExitingValidatorsQuery, ExitingValidatorsQueryVariables> = gql`
  query ExitingValidatorsQuery($first: Int = 100) {
    allHeadValidators(filter: { state: { in: [SLASHED_EXITING, EXITING] } }, orderBy: EXIT_EPOCH_ASC, first: $first) {
      nodes {
        balance
        effectiveBalance
        publicKey
        validatorIndex
        exitEpoch
        exitDate
        slashed
      }
    }
  }
`;
