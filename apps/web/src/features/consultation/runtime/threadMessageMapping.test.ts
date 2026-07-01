import { describe, expect, it } from 'vitest';
import { buildInterruptedTurnSeed, toInitialThreadMessage } from './threadMessageMapping';
import type { Message, PendingInteraction } from '../types/consultation';

function makeMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg-1',
    conversation_id: 'conv-1',
    turn_id: 'turn-1',
    role: 'assistant',
    status: 'completed',
    seq: 2,
    parts: [],
    content_text: '',
    model: null,
    provider: null,
    input_tokens: null,
    output_tokens: null,
    total_tokens: null,
    error: null,
    metadata: {},
    created_at: '2026-07-01T00:00:00.000Z',
    updated_at: '2026-07-01T00:00:00.000Z',
    ...overrides,
  };
}

describe('threadMessageMapping', () => {
  it('merges legacy tool_call and tool_result parts into one tool-call part', () => {
    const message = makeMessage({
      parts: [
        { type: 'tool_call', tool: 'search_knowledge', args: { query: '肩部前倾' } },
        { type: 'tool_result', tool: 'search_knowledge', result: { found: 2 } },
      ],
    });

    const mapped = toInitialThreadMessage(message);

    expect(mapped.role).toBe('assistant');
    expect(Array.isArray(mapped.content)).toBe(true);
    const content = mapped.content as Array<{ type: string; toolName?: string; result?: unknown }>;
    expect(content).toHaveLength(1);
    expect(content[0]).toMatchObject({
      type: 'tool-call',
      toolName: 'search_knowledge',
      result: { found: 2 },
    });
  });

  it('preserves structured source metadata for historical citations', () => {
    const message = makeMessage({
      parts: [
        {
          type: 'source',
          id: 'src-1',
          title: '头前伸自测方法',
          url: 'https://example.com/guide',
          providerMetadata: {
            bodysense: {
              summary: '判断耳垂与肩峰的相对位置',
            },
          },
        },
      ],
    });

    const mapped = toInitialThreadMessage(message);
    expect(Array.isArray(mapped.content)).toBe(true);
    const content = mapped.content as unknown as ReadonlyArray<{ type: string; providerMetadata?: unknown }>;

    expect(content[0]).toMatchObject({
      type: 'source',
      title: '头前伸自测方法',
      url: 'https://example.com/guide',
    });
    expect(content[0]?.providerMetadata).toEqual({
      bodysense: {
        summary: '判断耳垂与肩峰的相对位置',
      },
    });
  });

  it('maps aborted assistant messages to incomplete cancelled status', () => {
    const mapped = toInitialThreadMessage(
      makeMessage({
        status: 'aborted',
        parts: [{ type: 'text', text: '请先补充年龄信息。' }],
      }),
    );

    expect(mapped.status).toEqual({
      type: 'incomplete',
      reason: 'cancelled',
    });
  });

  it('rebuilds an interrupted active turn from an aborted message plus pending interaction', () => {
    const pendingInteraction: PendingInteraction = {
      id: 'int-1',
      run_id: 'run-1',
      conversation_id: 'conv-1',
      tool_call_id: 'tc-ask',
      tool_name: 'ask_user',
      question: {
        question: '你的年龄是多少？',
        answer_type: 'number',
        required: true,
      },
      status: 'pending',
      created_at: '2026-07-01T00:01:00.000Z',
    };

    const seed = buildInterruptedTurnSeed(
      [
        makeMessage({
          id: 'msg-aborted',
          status: 'aborted',
          parts: [
            { type: 'text', text: '为了继续分析，我还需要你的年龄。' },
            { type: 'tool_call', tool: 'search_knowledge', args: { query: '头前伸' }, tool_call_id: 'tc-1' },
            { type: 'tool_result', tool: 'search_knowledge', result: { found: 1 }, tool_call_id: 'tc-1' },
          ],
        }),
      ],
      [pendingInteraction],
    );

    expect(seed).not.toBeNull();
    expect(seed?.consumedMessageId).toBe('msg-aborted');
    expect(seed?.activeTurn.status).toBe('interrupted');
    expect(seed?.activeTurn.text).toContain('还需要你的年龄');
    expect(seed?.activeTurn.pendingInteraction?.id).toBe('int-1');
    expect(seed?.activeTurn.toolCallsById['tc-1']).toMatchObject({
      tool: 'search_knowledge',
      status: 'completed',
    });
  });
});
