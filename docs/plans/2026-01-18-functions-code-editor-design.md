# Edge Functions Code Editor Design

## Overview

Add a full-featured code editor to the sblite dashboard for editing edge function source files, similar to the Supabase dashboard experience.

## Requirements

- **Editor**: Monaco Editor (VS Code's editor engine)
- **Scope**: Full directory browser for each function
- **File operations**: Create, rename, delete files and folders
- **File types**: `.ts`, `.js`, `.json`, `.mjs`, `.tsx`, `.jsx`, `.html`, `.css`, `.md`, `.txt`
- **Auto-restart**: Prompt user to restart runtime after saving changes

## API Endpoints

New endpoints under `/_/api/functions/{name}/`:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/files` | List all files in function directory (recursive tree) |
| GET | `/files/*path` | Read file content |
| PUT | `/files/*path` | Create or update file |
| DELETE | `/files/*path` | Delete file or directory |
| POST | `/files/rename` | Rename/move file or directory |
| POST | `/restart` | Restart edge runtime |

### File Tree Response Format

```json
{
  "name": "hello-world",
  "children": [
    { "name": "index.ts", "type": "file", "size": 1024 },
    { "name": "utils", "type": "dir", "children": [
      { "name": "helpers.ts", "type": "file", "size": 512 }
    ]}
  ]
}
```

### Security Constraints

- Files restricted to allowed extensions only
- Path traversal protection (no `..` in paths, must stay within function directory)
- Maximum file size for editing: 1MB (larger files shown as "too large to edit")
- Hidden files (starting with `.`) excluded from listing

## UI Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ ← Back    hello-world    [Config ▼] [Test ▼] [⛶ Expand] [Delete]│
├───────────────┬─────────────────────────────────────────────────┤
│ Files         │ index.ts ●                      [Save] [⟳]      │
├───────────────┼─────────────────────────────────────────────────┤
│ ▼ hello-world │                                                 │
│   ▸ index.ts ●│  // hello-world edge function                   │
│   ▸ utils/    │  import { serve } from "...";                   │
│               │                                                 │
│ [+ File]      │  serve(async (req) => {                         │
│ [+ Folder]    │    ...                                          │
│               │  });                                            │
└───────────────┴─────────────────────────────────────────────────┘
```

### Key UI Elements

- **File tree (left panel)**: Shows directory structure with expand/collapse
- **Editor (right panel)**: Monaco editor with the selected file
- **Header bar**: File name, save button, restart button
- **Collapsible sections**: Config dropdown, Test console dropdown
- **Expand toggle**: Hides file tree for full-width editor

### Interactions

- Click file in tree → loads in editor
- Right-click file → context menu (rename, delete)
- `Ctrl+S` / `Cmd+S` → Save file
- Save triggers "Restart runtime?" confirmation dialog
- [⟳] button → Manual runtime restart
- Unsaved changes shown with dot indicator (●) on file name

## Implementation

### Monaco Editor Integration

- Load from CDN: `https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/`
- Languages: TypeScript, JavaScript, JSON, HTML, CSS, Markdown
- Theme: Match dashboard theme (auto dark/light mode support)
- Features: syntax highlighting, IntelliSense, bracket matching, minimap

### File Tree Component

- Custom vanilla JS tree component (no external dependency)
- Lazy loading for large directories
- File type icons using CSS or emoji
- Context menu on right-click: Rename, Delete, New File, New Folder

### State Management

```javascript
state.functions.editor = {
  currentFile: 'index.ts',      // Currently open file path
  content: '...',               // File content in editor
  originalContent: '...',       // For dirty detection
  isDirty: false,               // Has unsaved changes
  tree: { ... },                // File tree structure
  expandedFolders: ['utils'],   // Which folders are expanded
  isExpanded: false             // Full-width mode
}
```

### Backend Changes

1. Add file operation handlers in `internal/dashboard/handler.go`:
   - `handleListFunctionFiles` - recursive directory listing
   - `handleReadFunctionFile` - read file content with size check
   - `handleWriteFunctionFile` - create/update with extension validation
   - `handleDeleteFunctionFile` - delete with empty directory check
   - `handleRenameFunctionFile` - rename/move within function directory
   - `handleRestartFunctions` - restart edge runtime

2. Add path validation utility:
   - Sanitize paths (no `..`, no absolute paths)
   - Validate file extensions against allowlist
   - Check file size limits

3. Add restart endpoint:
   - Call `functionsService.Restart(ctx)`
   - Return success/error status

## File Structure Changes

```
internal/dashboard/
├── handler.go           # Add file operation handlers
├── static/
│   ├── app.js          # Add editor state and file tree
│   └── style.css       # Add editor styles
```

## Error Handling

| Scenario | Response |
|----------|----------|
| File not found | 404 with error message |
| Invalid extension | 400 "File type not allowed" |
| Path traversal attempt | 400 "Invalid path" |
| File too large | 400 "File too large to edit" |
| Runtime restart failed | 500 with error details |

## Testing Plan

1. Unit tests for path validation utility
2. API tests for all file operation endpoints
3. Manual testing of editor UI:
   - Create, edit, save, delete files
   - Create and delete folders
   - Rename files and folders
   - Restart runtime after changes
   - Dirty state detection
   - Keyboard shortcuts (Ctrl+S)
