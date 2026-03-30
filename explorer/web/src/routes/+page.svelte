<script>
  import { sendMessage } from '$lib/chat.js';

  let messages = $state([]);
  let input = $state('');
  let loading = $state(false);
  let messagesEl;

  function scrollToBottom() {
    if (messagesEl) {
      requestAnimationFrame(() => {
        messagesEl.scrollTop = messagesEl.scrollHeight;
      });
    }
  }

  async function send() {
    const text = input.trim();
    if (!text || loading) return;

    input = '';
    loading = true;

    messages.push({ role: 'user', content: text, parts: [] });
    messages.push({ role: 'assistant', content: '', parts: [] });
    scrollToBottom();

    const assistantIdx = messages.length - 1;

    await sendMessage(text, {
      onContent(content) {
        messages[assistantIdx].content += content;
        scrollToBottom();
      },
      onToolCall(name, args) {
        messages[assistantIdx].parts.push({ type: 'tool_call', name, args });
        scrollToBottom();
      },
      onDone() {
        loading = false;
        scrollToBottom();
      },
      onError(err) {
        messages[assistantIdx].content += `\n[Error: ${err}]`;
        loading = false;
        scrollToBottom();
      }
    });
  }

  function handleKeydown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }
</script>

<div class="explorer">
  <header>
    <h1>explorer</h1>
    <span class="subtitle">eval data, visualised</span>
  </header>

  <div class="messages" bind:this={messagesEl}>
    {#if messages.length === 0}
      <div class="empty">
        <p>Ask me about your eval results.</p>
        <p class="hint">Try: "Show me the latest BFCL runs" or "Compare accuracy across categories"</p>
      </div>
    {/if}

    {#each messages as msg}
      <div class="message {msg.role}">
        <div class="message-content">
          {#if msg.role === 'user'}
            <span class="label">you</span>
          {:else}
            <span class="label">explorer</span>
          {/if}
          <div class="text">{msg.content}</div>
          {#each msg.parts as part}
            {#if part.type === 'tool_call'}
              <div class="tool-call" data-tool={part.name}>
                <!-- chart renderers will mount here -->
                <span class="tool-pending">[{part.name}]</span>
              </div>
            {/if}
          {/each}
        </div>
      </div>
    {/each}

    {#if loading}
      <div class="thinking">thinking...</div>
    {/if}
  </div>

  <div class="input-area">
    <textarea
      bind:value={input}
      onkeydown={handleKeydown}
      placeholder="Ask about your eval data..."
      rows="1"
      disabled={loading}
    ></textarea>
    <button onclick={send} disabled={loading || !input.trim()}>
      send
    </button>
  </div>
</div>

<style>
  .explorer {
    display: flex;
    flex-direction: column;
    height: 100vh;
    max-width: 900px;
    margin: 0 auto;
    padding: 0 1rem;
  }

  header {
    padding: 1.5rem 0 1rem;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: baseline;
    gap: 1rem;
  }

  h1 {
    font-size: 1.2rem;
    font-weight: 600;
    color: var(--accent);
  }

  .subtitle {
    font-size: 0.8rem;
    color: var(--text-muted);
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 1rem 0;
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-muted);
    gap: 0.5rem;
  }

  .hint {
    font-size: 0.8rem;
    color: var(--text-muted);
    opacity: 0.6;
  }

  .message {
    margin-bottom: 1.5rem;
  }

  .label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin-bottom: 0.25rem;
    display: block;
  }

  .message.user .label { color: var(--text-secondary); }
  .message.assistant .label { color: var(--accent); }

  .text {
    white-space: pre-wrap;
    line-height: 1.6;
    font-size: 0.9rem;
  }

  .tool-call {
    margin: 0.75rem 0;
    padding: 1rem;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
  }

  .tool-pending {
    color: var(--text-muted);
    font-size: 0.8rem;
  }

  .thinking {
    color: var(--text-muted);
    font-size: 0.8rem;
    padding: 0.5rem 0;
    animation: pulse 1.5s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 1; }
  }

  .input-area {
    padding: 1rem 0;
    border-top: 1px solid var(--border);
    display: flex;
    gap: 0.5rem;
  }

  textarea {
    flex: 1;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-family: inherit;
    font-size: 0.9rem;
    padding: 0.75rem;
    resize: none;
    outline: none;
  }

  textarea:focus {
    border-color: var(--accent);
  }

  textarea:disabled {
    opacity: 0.5;
  }

  button {
    background: var(--accent);
    color: var(--bg-primary);
    border: none;
    border-radius: 4px;
    padding: 0.75rem 1.5rem;
    font-family: inherit;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
  }

  button:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  button:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
</style>
