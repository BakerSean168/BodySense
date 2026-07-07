import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { SessionCard } from '../SessionCard';
import type { Conversation } from '../../types/consultation';

function makeConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: 'conv-1',
    title: '腰背咨询',
    title_status: 'generated',
    status: 'active',
    pinned: false,
    pinned_at: null,
    default_model: null,
    last_message_at: null,
    message_count: 0,
    metadata: {},
    created_at: '2026-07-06T00:00:00Z',
    updated_at: '2026-07-06T00:00:00Z',
    ...overrides,
  };
}

describe('SessionCard', () => {
  it('prefetches on pointer, focus, and touch interactions', () => {
    const onPrefetch = vi.fn();
    const conversation = makeConversation();

    render(
      <SessionCard
        conversation={conversation}
        isActive={false}
        onPrefetch={onPrefetch}
        onSelect={vi.fn()}
        onDelete={vi.fn()}
        onPin={vi.fn()}
        onRename={vi.fn()}
        onShare={vi.fn()}
        onUnshare={vi.fn()}
      />,
    );

    const card = screen.getByTestId(`session-card-${conversation.id}`);
    fireEvent.pointerEnter(card);
    fireEvent.pointerDown(card);
    fireEvent.focus(card);
    fireEvent.touchStart(card);

    expect(onPrefetch).toHaveBeenCalledTimes(4);
  });
});
