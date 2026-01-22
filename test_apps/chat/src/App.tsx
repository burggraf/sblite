import { RealtimeChat } from '@/components/realtime-chat'

export function App() {
  return (
    <div className="h-screen w-screen">
      <RealtimeChat roomName="general" username="User" />
    </div>
  )
}

export default App
