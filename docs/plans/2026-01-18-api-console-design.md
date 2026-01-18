# API Console Design

## Overview

The API Console provides an interactive interface for testing all sblite APIs (REST, Auth, Admin). It uses a split-view layout with the request builder on the left and response viewer on the right.

## Layout

```
┌─────────────────────────────────────────────────────────────┐
│ API Console                                    [History ▼]  │
├─────────────────────────────┬───────────────────────────────┤
│ REQUEST                     │ RESPONSE                      │
│ ┌─────────┬───────────────┐ │ Status: 200 OK  Time: 45ms    │
│ │ POST ▼  │ /rest/v1/users│ │ ┌───────────────────────────┐ │
│ └─────────┴───────────────┘ │ │ Headers │ Body            │ │
│                             │ └───────────────────────────┘ │
│ [Templates ▼]               │ {                             │
│                             │   "id": "abc-123",            │
│ Headers:                    │   "email": "user@example.com" │
│ ┌─────────────────────────┐ │ }                             │
│ │ Authorization: Bearer...│ │                               │
│ └─────────────────────────┘ │                               │
│                             │                               │
│ Body:                       │                               │
│ ┌─────────────────────────┐ │                               │
│ │ { "email": "..." }      │ │                               │
│ └─────────────────────────┘ │                               │
│                             │                               │
│ [Send Request]              │                               │
└─────────────────────────────┴───────────────────────────────┘
```

API keys are auto-injected for REST and Auth API requests, with a toggle to enable/disable and select between `anon` and `service_role` keys.

## Request Builder

The left panel contains all request configuration:

### Method & URL
- Dropdown for HTTP method: GET, POST, PATCH, PUT, DELETE
- URL input field with base URL pre-filled
- URL autocomplete suggestions based on known endpoints

### Templates Dropdown
Pre-built templates organized by category:
- **Auth:** Sign Up, Sign In, Refresh Token, Get User, Update User, Logout
- **REST:** Select All, Select with Filter, Insert Row, Update Row, Delete Row
- **Admin:** List Tables, Create Table, Get Schema, Drop Table

Selecting a template populates method, URL, headers, and body with example values.

### Authentication Section
- Toggle to enable/disable API key auto-injection (enabled by default)
- Dropdown to select key type: `anon` or `service_role`
- API keys are auto-injected for `/rest/v1/` and `/auth/v1/` requests
- Keys are fetched from `/_/api/apikeys` endpoint on view load

### Headers Section
- Key-value pairs with add/remove buttons
- `Content-Type: application/json` added by default for POST/PATCH/PUT
- Can manually add any headers (apikey, Authorization, Prefer, Range, etc.)

### Body Section
- JSON editor textarea with basic formatting
- Only shown for POST, PATCH, PUT methods
- Placeholder text shows expected format based on selected template

### Send Button
- Executes request and displays response
- Shows loading spinner during request
- Disabled while request is in flight

## Response Viewer

The right panel displays the API response with status information:

### Status Bar
- HTTP status code with color coding (green for 2xx, yellow for 4xx, red for 5xx)
- Status text (e.g., "200 OK", "401 Unauthorized")
- Response time in milliseconds

### Tabs
- **Body** (default): Formatted JSON with syntax highlighting
- **Headers**: Response headers as key-value list

### Body Display
- Pretty-printed JSON with indentation
- Syntax highlighting (strings, numbers, booleans, null in different colors)
- Collapsible objects/arrays for large responses
- Copy button to copy raw JSON to clipboard
- Error messages displayed clearly for non-2xx responses

### Empty State
- Before any request: "Send a request to see the response"
- After error: Shows error message with suggestion

## Request History

History is persisted to localStorage and accessible via a dropdown in the header.

### History Dropdown
- Shows last 20 requests
- Each entry displays: method, URL path, status code, timestamp
- Color-coded by status (green/yellow/red)
- Click to reload request into builder
- "Clear History" button at bottom

### History Entry Format
```
POST /rest/v1/users     201  2 min ago
GET  /rest/v1/users     200  5 min ago
POST /auth/v1/token     401  10 min ago
```

### Storage Structure
```javascript
localStorage['sblite_api_console_history'] = JSON.stringify([
  {
    id: 'uuid',
    method: 'POST',
    url: '/rest/v1/users',
    headers: { ... },
    body: '{ ... }',
    status: 201,
    timestamp: 1705600000000
  }
])
```

### Behavior
- New requests added to top of history
- Duplicate consecutive requests not added
- History survives page refresh
- Clicking history item loads full request (method, URL, headers, body)
- Response is NOT stored (only request details)

## Implementation

### State Structure
```javascript
state.apiConsole = {
  method: 'GET',
  url: '/rest/v1/',
  headers: [{ key: 'Content-Type', value: 'application/json' }],
  body: '',
  response: null,        // { status, statusText, headers, body, time }
  loading: false,
  history: [],           // loaded from localStorage
  activeTab: 'body',     // 'body' or 'headers'
  showHistory: false,
  apiKeys: null,         // { anon_key, service_role_key }
  selectedKeyType: 'anon', // 'anon' or 'service_role'
  autoInjectKey: true    // auto-inject apikey header
}
```

### Backend Endpoint
- `GET /_/api/apikeys` - Returns `{ anon_key, service_role_key }` JWT tokens
  - Requires dashboard authentication
  - Keys are generated on-demand using the server's JWT secret

### Key Functions
- `loadApiConsoleHistory()` - Load from localStorage on init
- `saveApiConsoleHistory()` - Persist to localStorage
- `applyTemplate(name)` - Populate form from template
- `sendRequest()` - Execute fetch, measure time, update response
- `loadFromHistory(id)` - Restore request from history
- `renderApiConsole()` - Main view render
- `renderRequestBuilder()` - Left panel
- `renderResponseViewer()` - Right panel

### Templates Object
Pre-defined in JavaScript as object mapping template names to request configs.

## Testing

E2E tests in `e2e/tests/dashboard/api-console.test.ts`:

### API Console View
- navigates to API Console section
- displays split view layout
- shows method dropdown with all HTTP methods
- shows templates dropdown

### Request Builder
- selecting template populates form
- can edit URL manually
- can add custom headers
- body textarea shown only for POST/PATCH/PUT
- Authorization header auto-populated

### Sending Requests
- sends GET request and displays response
- sends POST request with body
- shows loading state during request
- displays error for failed requests
- shows response time

### Response Viewer
- displays formatted JSON body
- can switch to headers tab
- copy button copies response

### History
- request added to history after send
- clicking history item loads request
- history persists after page reload
- clear history removes all entries

### API Key Auto-Injection
- shows API key settings section
- auto-inject checkbox is enabled by default
- shows key type selector when auto-inject enabled
- REST API request succeeds with auto-injected key
- REST API request fails when auto-inject disabled
- can switch between anon and service_role key

26 tests covering the main functionality.
