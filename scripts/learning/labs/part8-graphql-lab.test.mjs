import assert from 'node:assert/strict';
import test from 'node:test';
import { gql } from '@apollo/client/core';
import {
  BODY_STATE_QUERY,
  createApolloCache,
  createGraphQLLab,
} from './part8-graphql-lab.mjs';

test('GraphQL returns only fields selected by the operation', async () => {
  const lab = createGraphQLLab({
    facts: [{ id: 'fact-1', kind: 'SYMPTOM', text: 'neck tightness' }],
  });

  const result = await lab.execute(`
    query RevisionOnly {
      bodyState {
        revision
      }
    }
  `);

  assert.deepEqual(result.errors, undefined);
  assert.equal(result.data.bodyState.revision, 1);
  assert.deepEqual(Object.keys(result.data.bodyState), ['revision']);
});

test('variables drive a mutation and the server returns authoritative state', async () => {
  const lab = createGraphQLLab();
  const result = await lab.execute(
    `
      mutation AddFact($input: AddFactInput!) {
        addFact(input: $input) {
          revision
          facts { id kind text }
        }
      }
    `,
    {
      input: {
        kind: 'CONTEXT',
        text: 'works at a desk',
        expectedRevision: 1,
      },
    },
  );

  assert.deepEqual(result.errors, undefined);
  assert.equal(result.data.addFact.revision, 2);
  assert.equal(lab.snapshot().revision, 2);
  assert.equal(result.data.addFact.facts[0].text, 'works at a desk');
});

test('domain conflicts are GraphQL execution errors, not schema-shape errors', async () => {
  const lab = createGraphQLLab({ revision: 3 });
  const result = await lab.execute(
    `
      mutation AddFact($input: AddFactInput!) {
        addFact(input: $input) { revision }
      }
    `,
    {
      input: {
        kind: 'SYMPTOM',
        text: 'left knee discomfort',
        expectedRevision: 2,
      },
    },
  );

  assert.equal(result.data, null);
  assert.equal(result.errors?.[0]?.extensions?.code, 'CONFLICT');
  assert.equal(result.errors?.[0]?.extensions?.currentRevision, 3);
  assert.equal(lab.snapshot().revision, 3);
});

test('Apollo normalized cache stores an entity once and projects it into queries', () => {
  const cache = createApolloCache();
  cache.writeQuery({
    query: BODY_STATE_QUERY,
    data: {
      bodyState: {
        __typename: 'BodyState',
        revision: 1,
        facts: [
          {
            __typename: 'BodyFact',
            id: 'fact-1',
            kind: 'SYMPTOM',
            text: 'neck tightness',
          },
        ],
      },
    },
  });

  const stored = cache.extract();
  assert.equal(stored['BodyFact:{"id":"fact-1"}'].text, 'neck tightness');

  cache.writeFragment({
    id: 'BodyFact:{"id":"fact-1"}',
    fragment: gql`
      fragment FactText on BodyFact {
        text
      }
    `,
    data: { text: 'neck tightness after sitting' },
  });

  const projected = cache.readQuery({ query: BODY_STATE_QUERY });
  assert.equal(projected.bodyState.facts[0].text, 'neck tightness after sitting');
});
