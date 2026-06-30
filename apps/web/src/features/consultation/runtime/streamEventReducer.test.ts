/**
 * Tests for StreamEventReducer — fixture-based, no backend dependencies.
 */

import { describe, it, expect } from 'vitest';
import {
  reduceStreamEvent,
  INITIAL_STATE,
} from './streamEventReducer';
import type { StreamEvent } from '../types/consultation';

// ---------------------------------------------------------------------------
// Fixtures — minimal StreamEvent shapes matching the contract
// ---------------------------------------------------------------------------

function makeEvent(
  type: string,
  payload: Record<string, unknown> = {},
  ids: Record<string, string> = {},
  channel = 'message',
): StreamEvent {
  return {
    version: 1,
    seq: 1,
    channel: channel as StreamEvent['channel'],
    type,
    ids: {
      conversation_id: ids.conversation_id || 'conv-1',
      run_id: ids.run_id || 'run-1',
      turn_id: ids.turn_id || 'turn-1',
      message_id: ids.message_id || 'msg-1',
      tool_call_id: ids.tool_call_id || null,
    },
    payload,
  } as StreamEvent;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('StreamEventReducer', () => {
  describe('message.text.delta', () => {
    it('appends delta text to assistantText', () => {
      const event = makeEvent('message.text.delta', { delta: 'Hello' });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.assistantText).toBe('Hello');
      expect(result.effects).toHaveLength(1);
      expect(result.effects[0]).toEqual({
        type: 'assistant_text_changed',
        text: 'Hello',
      });
    });

    it('accumulates multiple deltas', () => {
      const state1 = reduceStreamEvent(INITIAL_STATE, makeEvent('message.text.delta', { delta: 'Hello' }));
      const state2 = reduceStreamEvent(state1.state, makeEvent('message.text.delta', { delta: ' world' }));

      expect(state2.state.assistantText).toBe('Hello world');
    });
  });

  describe('state.extracted_info.upsert', () => {
    it('adds new body part', () => {
      const event = makeEvent('state.extracted_info.upsert', {
        info: { body_part: 'neck', symptom_type: 'pain' },
      });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.extractedInfo).toHaveLength(1);
      expect(result.state.extractedInfo[0].body_part).toBe('neck');
      expect(result.effects[0].type).toBe('extracted_info_updated');
    });

    it('merges existing body part by body_part key', () => {
      const state1 = reduceStreamEvent(
        INITIAL_STATE,
        makeEvent('state.extracted_info.upsert', {
          info: { body_part: 'neck', symptom_type: 'pain' },
        }),
      );
      const state2 = reduceStreamEvent(
        state1.state,
        makeEvent('state.extracted_info.upsert', {
          info: { body_part: 'neck', duration: '3 days' },
        }),
      );

      expect(state2.state.extractedInfo).toHaveLength(1);
      expect(state2.state.extractedInfo[0].body_part).toBe('neck');
      expect(state2.state.extractedInfo[0].symptom_type).toBe('pain');
      expect(state2.state.extractedInfo[0].duration).toBe('3 days');
    });

    it('keeps different body parts separate', () => {
      const state1 = reduceStreamEvent(
        INITIAL_STATE,
        makeEvent('state.extracted_info.upsert', {
          info: { body_part: 'neck', symptom_type: 'pain' },
        }),
      );
      const state2 = reduceStreamEvent(
        state1.state,
        makeEvent('state.extracted_info.upsert', {
          info: { body_part: 'back', symptom_type: 'stiffness' },
        }),
      );

      expect(state2.state.extractedInfo).toHaveLength(2);
    });
  });

  describe('source.citation.added', () => {
    it('adds citation', () => {
      const event = makeEvent('source.citation.added', {
        citation: { title: 'Spine Health', summary: 'Good posture' },
      });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.citations).toHaveLength(1);
      expect(result.state.citations[0].title).toBe('Spine Health');
      expect(result.effects[0].type).toBe('citation_added');
    });

    it('deduplicates by title', () => {
      const state1 = reduceStreamEvent(
        INITIAL_STATE,
        makeEvent('source.citation.added', {
          citation: { title: 'Spine Health', summary: 'First' },
        }),
      );
      const state2 = reduceStreamEvent(
        state1.state,
        makeEvent('source.citation.added', {
          citation: { title: 'Spine Health', summary: 'Second' },
        }),
      );

      expect(state2.state.citations).toHaveLength(1);
      // First citation is kept
      expect(state2.state.citations[0].summary).toBe('First');
    });
  });

  describe('safety.red_flag.detected', () => {
    it('sets red flags', () => {
      const event = makeEvent('safety.red_flag.detected', {
        has_red_flags: true,
        flags: [{ category: 'emergency', message: 'Seek care' }],
      });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.redFlags).not.toBeNull();
      expect(result.state.redFlags!.has_red_flags).toBe(true);
      expect(result.effects[0].type).toBe('red_flag');
    });
  });

  describe('source.knowledge_gap', () => {
    it('appends knowledge gap', () => {
      const event = makeEvent(
        'source.knowledge_gap',
        { query: 'posture exercises', message: 'No results found' },
        {},
        'source',
      );
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.knowledgeGaps).toHaveLength(1);
      expect(result.state.knowledgeGaps[0].query).toBe('posture exercises');
    });
  });

  describe('lifecycle events', () => {
    it('conversation.created sets conversationId and status', () => {
      const event = makeEvent(
        'conversation.created',
        { replaces_draft_id: 'draft-1' },
        { conversation_id: 'conv-new' },
        'conversation',
      );
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.conversationId).toBe('conv-new');
      expect(result.state.status).toBe('streaming');
      expect(result.effects[0].type).toBe('conversation_created');
    });

    it('message.persisted sets persistedUserMessageId', () => {
      const event = makeEvent(
        'message.persisted',
        { client_message_id: 'tmp-123', role: 'user' },
        { message_id: 'msg-persisted' },
      );
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.persistedUserMessageId).toBe('msg-persisted');
      expect(result.effects[0].type).toBe('message_persisted');
    });

    it('message.created sets assistantMessageId', () => {
      const event = makeEvent(
        'message.created',
        { role: 'assistant', status: 'streaming' },
        { message_id: 'msg-assistant' },
      );
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.assistantMessageId).toBe('msg-assistant');
      expect(result.state.status).toBe('streaming');
    });

    it('message.completed sets status to completed', () => {
      const event = makeEvent('message.completed', { status: 'completed', finish_reason: 'stop' });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.status).toBe('completed');
      expect(result.effects[0].type).toBe('message_completed');
    });

    it('message.failed sets status to failed with error', () => {
      const event = makeEvent('message.failed', {
        status: 'failed',
        error: { message: 'timeout' },
      });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.status).toBe('failed');
      expect(result.state.error).toBe('timeout');
    });

    it('stream.done sets status to completed', () => {
      const event = makeEvent('stream.done', {}, {}, 'stream');
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.status).toBe('completed');
    });

    it('stream.error sets status to failed', () => {
      const event = makeEvent('stream.error', { message: 'connection lost' }, {}, 'stream');
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state.status).toBe('failed');
      expect(result.state.error).toBe('connection lost');
      expect(result.effects[0].type).toBe('stream_error');
    });
  });

  describe('state.phase.changed', () => {
    it('emits phase_changed effect', () => {
      const event = makeEvent(
        'state.phase.changed',
        { from: 'collecting', to: 'ready_for_analysis', reason: 'enough info' },
        {},
        'state',
      );
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.effects[0]).toEqual({
        type: 'phase_changed',
        from: 'collecting',
        to: 'ready_for_analysis',
      });
    });
  });

  describe('unknown events', () => {
    it('treats unknown event types as no-op', () => {
      const event = makeEvent('unknown.event.type', { data: 'test' });
      const result = reduceStreamEvent(INITIAL_STATE, event);

      expect(result.state).toEqual(INITIAL_STATE);
      expect(result.effects).toHaveLength(0);
    });
  });

  describe('INITIAL_STATE', () => {
    it('has correct defaults', () => {
      expect(INITIAL_STATE.assistantText).toBe('');
      expect(INITIAL_STATE.extractedInfo).toEqual([]);
      expect(INITIAL_STATE.citations).toEqual([]);
      expect(INITIAL_STATE.redFlags).toBeNull();
      expect(INITIAL_STATE.knowledgeGaps).toEqual([]);
      expect(INITIAL_STATE.status).toBe('idle');
    });
  });
});
