import React, { useState, useMemo } from 'react';
import { marked, type Token, type Tokens } from 'marked';

export interface MarkdownRendererProps {
  content: string;
  isStreaming?: boolean;
  className?: string;
}

function isSafeUrl(url: string): boolean {
  if (!url) return false;
  if (url.startsWith('/') || url.startsWith('#')) return true;
  try {
    const parsed = new URL(url, typeof window !== 'undefined' ? window.location.href : 'https://localhost');
    return ['http:', 'https:', 'mailto:', 'tel:'].includes(parsed.protocol);
  } catch {
    return false;
  }
}

interface CodeBlockProps {
  code: string;
  lang?: string;
  hasCursor?: boolean;
}

export const CodeBlock: React.FC<CodeBlockProps> = ({ code, lang, hasCursor }) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      if (typeof navigator !== 'undefined' && navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else if (typeof document !== 'undefined') {
        const textArea = document.createElement('textarea');
        textArea.value = code;
        textArea.style.position = 'fixed';
        textArea.style.opacity = '0';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy code:', err);
    }
  };

  const displayLang = (lang || '').trim().toUpperCase() || 'CODE';

  return (
    <div className="md-code-block-wrapper">
      <div className="md-code-header">
        <span className="md-code-lang">{displayLang}</span>
        <button
          type="button"
          onClick={handleCopy}
          className={`md-code-copy-btn ${copied ? 'copied' : ''}`}
          aria-label="Copiar código al portapapeles"
        >
          {copied ? '✓ Copiado' : 'Copiar'}
        </button>
      </div>
      <pre className="md-pre">
        <code className="md-code">
          {code}
          {hasCursor && <span className="cursor-blink">▋</span>}
        </code>
      </pre>
    </div>
  );
};

function renderToken(
  token: Token,
  key: number,
  isLast: boolean,
  isStreaming: boolean
): React.ReactNode {
  switch (token.type) {
    case 'heading': {
      const headingToken = token as Tokens.Heading;
      const depth = Math.min(Math.max(headingToken.depth, 1), 6);
      const HeadingTag = `h${depth}` as 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';
      return (
        <HeadingTag key={key} className={`md-heading md-h${depth}`}>
          {renderTokens(headingToken.tokens || [{ type: 'text', raw: headingToken.text, text: headingToken.text }])}
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </HeadingTag>
      );
    }

    case 'paragraph': {
      const paragraphToken = token as Tokens.Paragraph;
      return (
        <p key={key} className="md-paragraph">
          {renderTokens(paragraphToken.tokens || [{ type: 'text', raw: paragraphToken.text, text: paragraphToken.text }])}
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </p>
      );
    }

    case 'code': {
      const codeToken = token as Tokens.Code;
      return (
        <CodeBlock
          key={key}
          code={codeToken.text}
          lang={codeToken.lang}
          hasCursor={isLast && isStreaming}
        />
      );
    }

    case 'blockquote': {
      const bqToken = token as Tokens.Blockquote;
      return (
        <blockquote key={key} className="md-blockquote">
          {renderTokens(bqToken.tokens, isLast, isStreaming)}
        </blockquote>
      );
    }

    case 'list': {
      const listToken = token as Tokens.List;
      const ListTag = listToken.ordered ? 'ol' : 'ul';
      const startProp =
        listToken.ordered && typeof listToken.start === 'number' && listToken.start !== 1
          ? { start: listToken.start }
          : {};

      return (
        <ListTag
          key={key}
          className={`md-list ${listToken.ordered ? 'md-list-ordered' : 'md-list-unordered'}`}
          {...startProp}
        >
          {listToken.items.map((item, itemIdx) => {
            const isLastItem = isLast && itemIdx === listToken.items.length - 1;
            const filteredTokens = (item.tokens || []).filter((t) => t.type !== 'checkbox');
            return (
              <li key={itemIdx} className={`md-list-item ${item.task ? 'md-task-item' : ''}`}>
                {item.task && (
                  <input
                    type="checkbox"
                    checked={item.checked}
                    readOnly
                    className="md-task-checkbox"
                    aria-label="Tarea"
                  />
                )}
                {renderTokens(
                  filteredTokens.length > 0 ? filteredTokens : [{ type: 'text', raw: item.text, text: item.text }],
                  isLastItem,
                  isStreaming
                )}
              </li>
            );
          })}
        </ListTag>
      );
    }

    case 'table': {
      const tableToken = token as Tokens.Table;
      return (
        <div key={key} className="md-table-wrapper">
          <table className="md-table">
            <thead>
              <tr>
                {tableToken.header.map((cell, headerIdx) => (
                  <th
                    key={headerIdx}
                    style={{ textAlign: (tableToken.align[headerIdx] as React.CSSProperties['textAlign']) || 'left' }}
                  >
                    {renderTokens(cell.tokens || [{ type: 'text', raw: cell.text, text: cell.text }])}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {tableToken.rows.map((row, rowIdx) => {
                const isLastRow = isLast && rowIdx === tableToken.rows.length - 1;
                return (
                  <tr key={rowIdx}>
                    {row.map((cell, cellIdx) => {
                      const isLastCell = isLastRow && cellIdx === row.length - 1;
                      return (
                        <td
                          key={cellIdx}
                          style={{
                            textAlign: (tableToken.align[cellIdx] as React.CSSProperties['textAlign']) || 'left',
                          }}
                        >
                          {renderTokens(cell.tokens || [{ type: 'text', raw: cell.text, text: cell.text }])}
                          {isLastCell && isStreaming && <span className="cursor-blink">▋</span>}
                        </td>
                      );
                    })}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      );
    }

    case 'hr': {
      return (
        <React.Fragment key={key}>
          <hr className="md-hr" />
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </React.Fragment>
      );
    }

    case 'space': {
      return null;
    }

    case 'strong': {
      const strongToken = token as Tokens.Strong;
      return (
        <strong key={key} className="md-strong">
          {renderTokens(strongToken.tokens || [{ type: 'text', raw: strongToken.text, text: strongToken.text }])}
        </strong>
      );
    }

    case 'em': {
      const emToken = token as Tokens.Em;
      return (
        <em key={key} className="md-em">
          {renderTokens(emToken.tokens || [{ type: 'text', raw: emToken.text, text: emToken.text }])}
        </em>
      );
    }

    case 'codespan': {
      const codeSpanToken = token as Tokens.Codespan;
      return (
        <code key={key} className="md-inline-code">
          {codeSpanToken.text}
        </code>
      );
    }

    case 'del': {
      const delToken = token as Tokens.Del;
      return (
        <del key={key} className="md-del">
          {renderTokens(delToken.tokens || [{ type: 'text', raw: delToken.text, text: delToken.text }])}
        </del>
      );
    }

    case 'link': {
      const linkToken = token as Tokens.Link;
      const safe = isSafeUrl(linkToken.href);
      return safe ? (
        <a
          key={key}
          href={linkToken.href}
          title={linkToken.title || undefined}
          target="_blank"
          rel="noopener noreferrer"
          className="md-link"
        >
          {renderTokens(linkToken.tokens || [{ type: 'text', raw: linkToken.text, text: linkToken.text }])}
        </a>
      ) : (
        <span key={key} className="md-link-disabled">
          {renderTokens(linkToken.tokens || [{ type: 'text', raw: linkToken.text, text: linkToken.text }])}
        </span>
      );
    }

    case 'image': {
      const imageToken = token as Tokens.Image;
      const safe = isSafeUrl(imageToken.href);
      return (
        <React.Fragment key={key}>
          {safe && (
            <img
              src={imageToken.href}
              alt={imageToken.text || imageToken.title || ''}
              title={imageToken.title || undefined}
              className="md-image"
              loading="lazy"
            />
          )}
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </React.Fragment>
      );
    }

    case 'br': {
      return <br key={key} />;
    }

    case 'checkbox': {
      const checkboxToken = token as Tokens.Checkbox;
      return (
        <input
          key={key}
          type="checkbox"
          checked={checkboxToken.checked}
          readOnly
          className="md-task-checkbox"
          aria-label="Tarea"
        />
      );
    }

    case 'text': {
      const textToken = token as Tokens.Text;
      if (textToken.tokens && textToken.tokens.length > 0) {
        return (
          <React.Fragment key={key}>
            {renderTokens(textToken.tokens)}
            {isLast && isStreaming && <span className="cursor-blink">▋</span>}
          </React.Fragment>
        );
      }
      return (
        <React.Fragment key={key}>
          {textToken.text}
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </React.Fragment>
      );
    }

    case 'escape': {
      const escapeToken = token as Tokens.Escape;
      return (
        <React.Fragment key={key}>
          {escapeToken.text}
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </React.Fragment>
      );
    }

    case 'html': {
      const htmlToken = token as Tokens.HTML;
      return (
        <React.Fragment key={key}>
          <span className="md-html-text">{htmlToken.text}</span>
          {isLast && isStreaming && <span className="cursor-blink">▋</span>}
        </React.Fragment>
      );
    }

    case 'def': {
      return null;
    }

    default: {
      const genToken = token as unknown as { tokens?: Token[]; text?: string };
      if (genToken.tokens && Array.isArray(genToken.tokens)) {
        return (
          <span key={key}>
            {renderTokens(genToken.tokens)}
            {isLast && isStreaming && <span className="cursor-blink">▋</span>}
          </span>
        );
      }
      if (typeof genToken.text === 'string') {
        return (
          <React.Fragment key={key}>
            {genToken.text}
            {isLast && isStreaming && <span className="cursor-blink">▋</span>}
          </React.Fragment>
        );
      }
      return null;
    }
  }
}

function renderTokens(
  tokens: Token[] | undefined,
  isLastContext = false,
  isStreaming = false
): React.ReactNode {
  if (!tokens || tokens.length === 0) return null;

  const nonSpaceIndices: number[] = [];
  tokens.forEach((t, i) => {
    if (t.type !== 'space') nonSpaceIndices.push(i);
  });
  const lastActiveIndex = nonSpaceIndices.length > 0 ? nonSpaceIndices[nonSpaceIndices.length - 1] : -1;

  return tokens.map((token, index) => {
    const isLastToken = isLastContext && index === lastActiveIndex;
    return renderToken(token, index, isLastToken, isStreaming);
  });
}

export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({
  content,
  isStreaming = false,
  className = '',
}) => {
  const tokens = useMemo(() => {
    if (!content) return [];
    try {
      return marked.lexer(content, { gfm: true, breaks: true });
    } catch (err) {
      console.error('Error parsing markdown:', err);
      return [];
    }
  }, [content]);

  if (!content.trim()) {
    return (
      <div className={`markdown-content ${className}`}>
        {isStreaming && <span className="cursor-blink">▋</span>}
      </div>
    );
  }

  return (
    <div className={`markdown-content ${className}`}>
      {renderTokens(tokens, true, isStreaming)}
    </div>
  );
};
