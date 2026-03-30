/**
 * Send a message to the explorer API and stream the response.
 * Calls handlers for each SSE event type: content, tool_call, done, error.
 */
export async function sendMessage(message, handlers) {
  const resp = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message })
  });

  if (!resp.ok) {
    const err = await resp.text();
    handlers.onError?.(err);
    return;
  }

  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      if (!line.startsWith('data: ')) continue;
      const json = line.slice(6);
      try {
        const event = JSON.parse(json);
        switch (event.type) {
          case 'content':
            handlers.onContent?.(event.content);
            break;
          case 'tool_call':
            handlers.onToolCall?.(event.name, event.arguments);
            break;
          case 'done':
            handlers.onDone?.();
            break;
          case 'error':
            handlers.onError?.(event.error);
            break;
        }
      } catch (e) {
        // skip malformed events
      }
    }
  }
}
