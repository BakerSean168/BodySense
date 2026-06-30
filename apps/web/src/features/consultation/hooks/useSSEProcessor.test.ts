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
});
