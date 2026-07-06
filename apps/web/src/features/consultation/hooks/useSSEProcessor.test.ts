import { describe, expect, it, vi } from 'vitest';
import { processSSELine, type SSEHandlers } from './useSSEProcessor';

describe('processSSELine', () => {
  it('dispatches structured text delta events by envelope type', () => {
    const onTextDelta = vi.fn();
    const handlers: SSEHandlers = { onTextDelta };
    const state = { currentEvent: '' };

    processSSELine('event: message.text.delta', state, handlers);
    processSSELine(
      'data: {"version":1,"seq":1,"channel":"message","type":"message.text.delta","ids":{"message_id":"m1"},"payload":{"delta":"hello"}}',
      state,
      handlers,
    );

    expect(onTextDelta).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'message.text.delta',
        payload: { delta: 'hello' },
      }),
    );
  });

  it('dispatches stream.done as a structured event', () => {
    const onDone = vi.fn();
    const handlers: SSEHandlers = { onDone };
    const state = { currentEvent: '' };

    processSSELine('event: stream.done', state, handlers);
    processSSELine(
      'data: {"version":1,"seq":2,"channel":"stream","type":"stream.done","ids":{},"payload":{}}',
      state,
      handlers,
    );

    expect(onDone).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'stream.done',
        payload: {},
      }),
    );
  });

  it('dispatches state.health_features.upsert events', () => {
    const onHealthFeatures = vi.fn();
    const handlers: SSEHandlers = { onHealthFeatures };
    const state = { currentEvent: '' };

    processSSELine('event: state.health_features.upsert', state, handlers);
    processSSELine(
      'data: {"version":1,"seq":3,"channel":"state","type":"state.health_features.upsert","ids":{"conversation_id":"c1"},"payload":{"health_features":{"posture_findings":[{"label":"头前移"}],"discomforts":[],"negative_findings":[],"movement_limitations":[],"red_flags":[],"user_answers":[]}}}',
      state,
      handlers,
    );

    expect(onHealthFeatures).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'state.health_features.upsert',
        payload: expect.objectContaining({
          health_features: expect.objectContaining({
            posture_findings: [{ label: '头前移' }],
          }),
        }),
      }),
    );
  });
});
