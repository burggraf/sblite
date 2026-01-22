# Chat Test App

A realtime chat application demonstrating sblite's Realtime Broadcast feature using the official `@supabase/supabase-js` client.

## Purpose

This test app validates that sblite's realtime WebSocket implementation is compatible with Supabase's client library. It uses the **Broadcast** feature to send and receive messages between multiple browser tabs/windows in real-time.

## Stack

- **React 19** - UI framework
- **TypeScript** - Type safety
- **Vite 7** - Build tool and dev server
- **Tailwind CSS v4** - Styling
- **shadcn/ui** - UI components
- **@supabase/supabase-js** - Supabase client (connects to sblite)

## Prerequisites

1. **sblite** must be built and initialized:
   ```bash
   # From the sblite root directory
   go build -o sblite .
   ./sblite init
   ```

2. **Node.js 18+** for running the Vite dev server

## Setup

1. Install dependencies:
   ```bash
   cd test_apps/chat
   npm install
   ```

2. Configure environment variables:

   Copy `.env.local` and set your anon key:
   ```bash
   VITE_SUPABASE_URL=http://localhost:8080
   VITE_SUPABASE_PUBLISHABLE_OR_ANON_KEY=<your_anon_key>
   ```

   You can find your anon key by:
   - Opening the sblite dashboard at `http://localhost:8080/_` and navigating to Settings > API Keys
   - Or running: `./sblite serve` and checking the startup logs

## Running

1. **Start sblite with realtime enabled** (required):
   ```bash
   # From the sblite root directory
   ./sblite serve --realtime
   ```

2. **Start the chat app**:
   ```bash
   # From test_apps/chat
   npm run dev
   ```

3. Open `http://localhost:5173` in multiple browser tabs to test realtime messaging between clients.

## How It Works

The app uses Supabase's realtime broadcast feature:

```typescript
// Subscribe to a channel
const channel = supabase.channel('general')

channel
  .on('broadcast', { event: 'message' }, (payload) => {
    // Receive messages from other clients
  })
  .subscribe()

// Send a message
channel.send({
  type: 'broadcast',
  event: 'message',
  payload: { content: 'Hello!' }
})
```

Messages are broadcast to all connected clients in the same channel. No database persistence - messages only exist in memory during the session.

## Project Structure

```
chat/
├── src/
│   ├── components/
│   │   ├── realtime-chat.tsx    # Main chat UI component
│   │   ├── chat-message.tsx     # Individual message display
│   │   └── ui/                  # shadcn/ui components
│   ├── hooks/
│   │   ├── use-realtime-chat.tsx  # Supabase realtime hook
│   │   └── use-chat-scroll.tsx    # Auto-scroll behavior
│   ├── lib/
│   │   └── supabase/
│   │       └── client.ts        # Supabase client instance
│   ├── App.tsx                  # Root component
│   └── main.tsx                 # Entry point
├── .env.local                   # Environment config
└── package.json
```
