import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MarkdownRenderer } from '../components/MarkdownRenderer';
import { ConversationPanel, ChatMessage } from '../components/ConversationPanel';

describe('MarkdownRenderer', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders headings correctly', () => {
    const markdown = '# Heading 1\n## Heading 2\n### Heading 3';
    render(<MarkdownRenderer content={markdown} />);

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Heading 1');
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Heading 2');
    expect(screen.getByRole('heading', { level: 3 })).toHaveTextContent('Heading 3');
  });

  it('renders inline formatting (bold, italic, strikethrough, inline code)', () => {
    const markdown = 'This is **bold**, *italic*, ~~strikethrough~~, and `inline code`.';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const strong = container.querySelector('strong');
    expect(strong).toHaveTextContent('bold');

    const em = container.querySelector('em');
    expect(em).toHaveTextContent('italic');

    const del = container.querySelector('del');
    expect(del).toHaveTextContent('strikethrough');

    const code = container.querySelector('code.md-inline-code');
    expect(code).toHaveTextContent('inline code');
  });

  it('renders unordered and ordered lists', () => {
    const markdown = '- Bullet 1\n- Bullet 2\n\n1. Numbered 1\n2. Numbered 2';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const ul = container.querySelector('ul.md-list-unordered');
    expect(ul).toBeTruthy();
    expect(ul?.querySelectorAll('li')).toHaveLength(2);

    const ol = container.querySelector('ol.md-list-ordered');
    expect(ol).toBeTruthy();
    expect(ol?.querySelectorAll('li')).toHaveLength(2);
  });

  it('renders task list checkboxes', () => {
    const markdown = '- [x] Completed task\n- [ ] Pending task';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const checkboxes = container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
    expect(checkboxes).toHaveLength(2);
    expect(checkboxes[0].checked).toBe(true);
    expect(checkboxes[1].checked).toBe(false);
  });

  it('renders blockquotes', () => {
    const markdown = '> This is a quote from Jarvis';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const bq = container.querySelector('blockquote');
    expect(bq).toBeTruthy();
    expect(bq).toHaveTextContent('This is a quote from Jarvis');
  });

  it('renders tables properly', () => {
    const markdown = `
| Name | Type |
| :--- | :--- |
| id   | uuid |
| user | text |
    `.trim();
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const table = container.querySelector('table.md-table');
    expect(table).toBeTruthy();
    const headers = container.querySelectorAll('th');
    expect(headers).toHaveLength(2);
    expect(headers[0]).toHaveTextContent('Name');
    expect(headers[1]).toHaveTextContent('Type');

    const cells = container.querySelectorAll('td');
    expect(cells).toHaveLength(4);
    expect(cells[0]).toHaveTextContent('id');
    expect(cells[1]).toHaveTextContent('uuid');
  });

  it('renders safe links with target _blank and blocks javascript: links', () => {
    const markdown = '[Good Link](https://example.com) and [Bad Link](javascript:alert(1))';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    const goodLink = container.querySelector('a[href="https://example.com"]');
    expect(goodLink).toBeTruthy();
    expect(goodLink?.getAttribute('target')).toBe('_blank');
    expect(goodLink?.getAttribute('rel')).toBe('noopener noreferrer');

    const badLink = container.querySelector('a[href^="javascript"]');
    expect(badLink).toBeNull();
    const disabledLink = container.querySelector('.md-link-disabled');
    expect(disabledLink).toHaveTextContent('Bad Link');
  });

  it('prevents XSS from raw HTML tags', () => {
    const markdown = '<script>window.pwned = true;</script>';
    const { container } = render(<MarkdownRenderer content={markdown} />);

    expect(container.querySelector('script')).toBeNull();
    expect(container.textContent).toContain("<script>window.pwned = true;</script>");
  });

  it('renders code blocks with copy functionality', async () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    });

    const markdown = '```typescript\nconst message = "Hello Jarvis";\nconsole.log(message);\n```';
    render(<MarkdownRenderer content={markdown} />);

    expect(screen.getByText('TYPESCRIPT')).toBeTruthy();
    expect(screen.getByText(/const message = "Hello Jarvis";/)).toBeTruthy();

    const copyBtn = screen.getByRole('button', { name: /copiar/i });
    expect(copyBtn).toHaveTextContent('Copiar');

    fireEvent.click(copyBtn);
    expect(writeTextMock).toHaveBeenCalledWith('const message = "Hello Jarvis";\nconsole.log(message);');

    await waitFor(() => {
      expect(copyBtn).toHaveTextContent('✓ Copiado');
    });
  });

  it('displays streaming cursor when isStreaming is true', () => {
    const { container, rerender } = render(<MarkdownRenderer content="" isStreaming={true} />);
    expect(container.querySelector('.cursor-blink')).toBeTruthy();

    rerender(<MarkdownRenderer content="Generando respuesta..." isStreaming={true} />);
    expect(container.querySelector('.cursor-blink')).toBeTruthy();
    expect(container.textContent).toContain('Generando respuesta...');

    rerender(<MarkdownRenderer content="Respuesta completa." isStreaming={false} />);
    expect(container.querySelector('.cursor-blink')).toBeNull();
  });
});

describe('ConversationPanel markdown integration', () => {
  it('renders assistant responses with formatted markdown instead of raw syntax', () => {
    const messages: ChatMessage[] = [
      {
        id: 'msg-1',
        role: 'assistant',
        content: '### Resultado de la consulta\nEncontré los siguientes registros:\n- **Registro 1**: `Activo`\n- **Registro 2**: `Pendiente`\n\n```sql\nSELECT * FROM logs;\n```',
        timestamp: Date.now(),
      },
    ];

    const { container } = render(
      <ConversationPanel
        isOpen={true}
        onClose={vi.fn()}
        privacyMode="strict-private"
        onModeSwitchRequest={vi.fn()}
        onSendMessage={vi.fn()}
        messages={messages}
        status="IDLE"
      />
    );

    // Checks that markdown heading 3 is rendered as <h3>
    const h3 = container.querySelector('h3.md-heading');
    expect(h3).toBeTruthy();
    expect(h3).toHaveTextContent('Resultado de la consulta');

    // Checks that bold is rendered as <strong>
    const strong = container.querySelector('strong.md-strong');
    expect(strong).toHaveTextContent('Registro 1');

    // Checks that inline code is rendered as <code>
    const inlineCodes = container.querySelectorAll('code.md-inline-code');
    expect(inlineCodes.length).toBeGreaterThanOrEqual(2);
    expect(inlineCodes[0]).toHaveTextContent('Activo');

    // Checks that code block is rendered as pre/code with copy button
    const codeBlock = container.querySelector('.md-code-block-wrapper');
    expect(codeBlock).toBeTruthy();
    expect(codeBlock).toHaveTextContent('SQL');
    expect(codeBlock).toHaveTextContent('SELECT * FROM logs;');

    // Checks that raw markdown characters like ### or ```sql are NOT displayed as plain unformatted text
    expect(container.textContent).not.toContain('### Resultado');
    expect(container.textContent).not.toContain('```sql');
  });
});
