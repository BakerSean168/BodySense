import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, fn } from 'storybook/test';
import { Button } from './Button';

const meta = {
  title: 'UI/Button',
  component: Button,
  tags: ['autodocs'],
} satisfies Meta<typeof Button>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: '开始诊断',
  },
};

export const Secondary: Story = {
    args: {
        children: '开始诊断',
        variant: 'secondary',
    },
    };

    export const Outline: Story = {
    args: {
        children: '开始诊断',
        variant: 'outline',
    },
    };

    export const Ghost: Story = {
    args: {
        children: '开始诊断',
        variant: 'ghost',
    },
    };

    // 危险操作按钮
export const Destructive: Story = {
  args: {
    variant: 'destructive',
    children: '删除诊断',
  },
};

// 不同尺寸
export const Small: Story = {
  args: {
    size: 'sm',
    children: '开始诊断',
  },
};

export const Large: Story = {
  args: {
    size: 'lg',
    children: '开始诊断',
  },
};

// 加载状态
export const Loading: Story = {
  args: {
    isLoading: true,
    children: '正在保存...',
  },
};

export const Disabled: Story = {
  args: {
    children: '开始诊断',
    disabled: true,
  },
};

// 交互测试
export const ClickTest: Story = {
  args: {
    onClick: fn(),
    children: '点击测试',
  },

  play: async ({ canvas, args }) => {
    const button = canvas.getByRole('button', {
      name: '点击测试',
    });

    await userEvent.click(button);

    await expect(args.onClick).toHaveBeenCalledOnce();
  },
};
