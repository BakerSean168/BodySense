import { ApolloServer } from '@apollo/server';
import { InMemoryCache, gql } from '@apollo/client/core';
import { GraphQLError } from 'graphql';

export const typeDefs = `
  enum FactKind {
    SYMPTOM
    CONTEXT
  }

  type BodyFact {
    id: ID!
    kind: FactKind!
    text: String!
  }

  type BodyState {
    revision: Int!
    facts: [BodyFact!]!
  }

  input AddFactInput {
    kind: FactKind!
    text: String!
    expectedRevision: Int!
  }

  type Query {
    bodyState: BodyState!
  }

  type Mutation {
    addFact(input: AddFactInput!): BodyState!
  }
`;

export function createGraphQLLab(initial = {}) {
  let state = {
    revision: initial.revision ?? 1,
    facts: initial.facts ? structuredClone(initial.facts) : [],
  };

  const server = new ApolloServer({
    typeDefs,
    resolvers: {
      Query: {
        bodyState: () => structuredClone(state),
      },
      Mutation: {
        addFact: (_parent, { input }) => {
          if (input.expectedRevision !== state.revision) {
            throw new GraphQLError('stale body state revision', {
              extensions: {
                code: 'CONFLICT',
                currentRevision: state.revision,
              },
            });
          }

          const fact = {
            id: `fact-${state.facts.length + 1}`,
            kind: input.kind,
            text: input.text,
          };
          state = {
            revision: state.revision + 1,
            facts: [...state.facts, fact],
          };
          return structuredClone(state);
        },
      },
    },
  });

  return {
    async execute(source, variableValues) {
      const response = await server.executeOperation({
        query: source,
        variables: variableValues,
      });
      if (response.body.kind !== 'single') {
        throw new Error('Part 8 lab expects a single GraphQL operation result');
      }
      return response.body.singleResult;
    },
    snapshot() {
      return structuredClone(state);
    },
  };
}

export const BODY_STATE_QUERY = gql`
  query BodyStateForWorkbench {
    bodyState {
      revision
      facts {
        id
        kind
        text
      }
    }
  }
`;

export function createApolloCache() {
  return new InMemoryCache({
    typePolicies: {
      BodyFact: {
        keyFields: ['id'],
      },
    },
  });
}
