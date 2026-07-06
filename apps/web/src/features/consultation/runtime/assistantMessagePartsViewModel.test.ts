import { describe, expect, it } from 'vitest';
import type { ThreadAssistantMessagePart } from '@assistant-ui/react';
import { buildAssistantMessagePartsViewModel } from './assistantMessagePartsViewModel';

describe('assistantMessagePartsViewModel', () => {
  it('extracts text, tools, citations, red flags, and knowledge gaps from assistant parts', () => {
    const parts: ThreadAssistantMessagePart[] = [
      { type: 'text', text: '根据资料，建议先做侧面自拍。' },
      {
        type: 'tool-call',
        toolCallId: 'tc-1',
        toolName: 'search_knowledge',
        args: { query: '头前伸自测' },
        argsText: '{"query":"头前伸自测"}',
        result: { found: 1 },
      },
      {
        type: 'source',
        sourceType: 'url',
        id: 'src-1',
        url: 'https://example.com/guide',
        title: '头前伸自测方法',
        providerMetadata: {
          bodysense: {
            summary: '判断耳垂与肩峰的关系',
          },
        },
      },
      {
        type: 'data',
        name: 'knowledge_gap',
        data: { query: '斜方肌紧张', message: '暂无专项资料' },
      },
      {
        type: 'data',
        name: 'red_flag',
        data: {
          has_red_flags: true,
          flags: [{ category: 'emergency', message: '剧烈疼痛', matched_text: '', source: '' }],
        },
      },
    ];

    const vm = buildAssistantMessagePartsViewModel(parts);

    expect(vm.markdown).toContain('建议先做侧面自拍');
    expect(vm.toolCalls).toHaveLength(1);
    expect(vm.citations[0]).toMatchObject({
      title: '头前伸自测方法',
      summary: '判断耳垂与肩峰的关系',
    });
    expect(vm.knowledgeGaps).toEqual([
      { query: '斜方肌紧张', message: '暂无专项资料' },
    ]);
    expect(vm.redFlag?.has_red_flags).toBe(true);
    expect(vm.hasRenderableContent).toBe(true);
  });

  it('treats ask_user-only assistant parts as non-renderable', () => {
    const parts: ThreadAssistantMessagePart[] = [
      {
        type: 'tool-call',
        toolCallId: 'tc-ask',
        toolName: 'ask_user',
        args: { question: '是否有颈肩不适？' },
        argsText: '{"question":"是否有颈肩不适？"}',
      },
      {
        type: 'tool-call',
        toolCallId: 'tc-ask',
        toolName: 'ask_user',
        args: { question: '是否有颈肩不适？' },
        argsText: '{"question":"是否有颈肩不适？"}',
        result: { answer: { text: '无' } },
      },
    ];

    const vm = buildAssistantMessagePartsViewModel(parts);

    expect(vm.toolCalls).toHaveLength(0);
    expect(vm.hasRenderableContent).toBe(false);
  });
});
