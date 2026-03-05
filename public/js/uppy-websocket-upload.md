# Materials Commons WebSocket Upload Plugin for Uppy

An Uppy plugin that implements the Materials Commons WebSocket-based resumable file upload protocol.

## Overview

This plugin connects to the Materials Commons WebSocket server and uploads files using a chunked, resumable protocol. It implements the protocol defined in `pkg/mctus2/wserv/client_connection.go`.

## Features

- **WebSocket-based uploads**: Efficient bidirectional communication with the server
- **Chunked transfers**: Files are split into chunks and uploaded with a sliding window protocol
- **Sliding window protocol**: Sends multiple chunks (default: 10) before waiting for ACKs, dramatically improving upload speed
- **Resumable uploads**: Interrupted uploads can be resumed from where they left off
- **Progress tracking**: Real-time progress updates during upload
- **Automatic reconnection**: Connection drops are handled with exponential backoff
- **Checksum verification**: MD5 checksums ensure file integrity
- **Multiple file support**: Upload multiple files concurrently

## Protocol Implementation

The plugin implements the following message flow:

### 1. Connection
- Establishes WebSocket connection to server
- Sends periodic heartbeat messages to keep connection alive
- Handles reconnection on connection drops

### 2. Transfer Initialization
```javascript
// Client sends TRANSFER_INIT
{
  "command": "TRANSFER_INIT",
  "id": "transfer_id",
  "payload": {
    "transfer_id": "unique_id",
    "project_id": 123,
    "project_path": "/path/in/project",
    "file_path": "local_file.txt",
    "file_size": 1048576,
    "chunk_size": 5242880,
    "checksum": "md5_hash"
  }
}

// Server responds with TRANSFER_ACCEPT or TRANSFER_REJECT
{
  "command": "TRANSFER_ACCEPT",
  "payload": {
    "transfer_id": "unique_id",
    "chunk_size": 5242880,
    "expected_chunks": 20
  }
}
```

### 3. Chunk Upload (Sliding Window)
The plugin uses a **sliding window protocol** to upload chunks efficiently:

- Sends up to `windowSize` chunks (default: 10) without waiting for individual ACKs
- Tracks "in-flight" chunks that have been sent but not yet acknowledged
- When ACK is received, removes chunk from window and sends next chunk to keep window full
- Only pauses sending when window is full (10 unacknowledged chunks)

Binary WebSocket messages with format:
```
{"transfer_id":"...", "sequence":0, "size":5242880, "is_last":false}\n
[binary chunk data]
```

Server responds with `CHUNK_ACK` after each chunk (asynchronously):
```javascript
{
  "command": "CHUNK_ACK",
  "payload": {
    "transfer_id": "unique_id",
    "chunk_sequence": 0,
    "bytes_received": 5242880,
    "next_sequence": 1
  }
}
```

**Example flow:**
- Send chunks 0-9 immediately (window full)
- Wait for ACKs to start arriving
- As ACK for chunk 0 arrives → send chunk 10
- As ACK for chunk 1 arrives → send chunk 11
- Continue until all chunks sent and acknowledged

### 4. Transfer Completion
```javascript
// Client sends TRANSFER_COMPLETE
{
  "command": "TRANSFER_COMPLETE",
  "payload": {
    "transfer_id": "unique_id"
  }
}

// Server responds with TRANSFER_FINALIZE
{
  "command": "TRANSFER_FINALIZE",
  "payload": {
    "transfer_id": "unique_id",
    "status": "complete",
    "bytes_written": 1048576
  }
}
```

### 5. Resume Support
```javascript
// Client sends TRANSFER_RESUME
{
  "command": "TRANSFER_RESUME",
  "payload": {
    "transfer_id": "existing_id"
  }
}

// Server responds with TRANSFER_RESUME_RESPONSE
{
  "command": "TRANSFER_RESUME_RESPONSE",
  "payload": {
    "transfer_id": "existing_id",
    "can_resume": true,
    "resume_from_byte": 10485760,
    "resume_from_chunk": 2,
    "bytes_received": 10485760
  }
}
```

## Usage

### Installation

```bash
npm install @uppy/core cuid
```

Include the plugin:
```html
<script src="uppy-websocket-upload.js"></script>
```

### Basic Example

```javascript
const uppy = new Uppy.Core({
  autoProceed: false
})

// Add Dashboard UI
uppy.use(Uppy.Dashboard, {
  inline: true,
  target: '#uppy'
})

// Add WebSocket Upload plugin
uppy.use(MaterialsCommonsWebSocketUpload, {
  serverUrl: 'ws://localhost:1352/ws',
  chunkSize: 5 * 1024 * 1024,  // 5MB chunks
  windowSize: 10                // Send 10 chunks before waiting for ACKs
})

// Set file metadata before upload
uppy.on('file-added', (file) => {
  uppy.setFileMeta(file.id, {
    projectId: 123,
    projectPath: '/uploads/' + file.name,
    filePath: file.name
  })
})
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `serverUrl` | String | `ws://localhost:1352/ws` | WebSocket server URL |
| `chunkSize` | Number | `5242880` (5MB) | Size of each chunk in bytes |
| `windowSize` | Number | `10` | Number of chunks to send before waiting for ACKs |
| `clientId` | String | Auto-generated | Unique client identifier |
| `reconnectDelay` | Number | `1000` | Initial reconnection delay in ms |
| `maxReconnectDelay` | Number | `30000` | Maximum reconnection delay in ms |
| `heartbeatInterval` | Number | `30000` | Heartbeat interval in ms |

### File Metadata

Each file must have the following metadata set before upload:

- `projectId` (required): The Materials Commons project ID
- `projectPath` (required): The destination path within the project
- `filePath` (optional): The local file path (defaults to filename)

Example:
```javascript
uppy.setFileMeta(fileId, {
  projectId: 123,
  projectPath: '/data/experiment1/results.csv',
  filePath: 'results.csv'
})
```

## Events

The plugin emits standard Uppy events:

```javascript
uppy.on('upload-started', (file) => {
  console.log('Upload started:', file.name)
})

uppy.on('upload-progress', (file, progress) => {
  console.log(`Progress: ${progress.percentage}%`)
})

uppy.on('upload-success', (file, response) => {
  console.log('Upload complete:', response)
})

uppy.on('upload-error', (file, error) => {
  console.error('Upload error:', error)
})

uppy.on('complete', (result) => {
  console.log('All uploads complete:', result)
})
```

## Advanced Usage

### Cancel Upload

```javascript
const plugin = uppy.getPlugin('MaterialsCommonsWebSocketUpload')
plugin.cancelTransfer(transferId)
```

### Manual Connection Control

```javascript
const plugin = uppy.getPlugin('MaterialsCommonsWebSocketUpload')

// Disconnect
plugin.disconnect()

// Reconnect
plugin.connect()
```

### Resume Interrupted Uploads

The plugin automatically attempts to resume interrupted uploads using localStorage. Transfer state is saved after each chunk and can be resumed across browser sessions.

## Testing

Open `uppy-websocket-upload-example.html` in a browser to test the plugin:

1. Configure the WebSocket server URL
2. Set the project ID and destination path
3. Select files to upload
4. Monitor progress and check browser console for detailed logs

## Dependencies

- **@uppy/core**: Uppy framework
- **cuid**: For generating unique transfer IDs
- **CryptoJS** (optional): For MD5 checksum calculation (falls back to Web Crypto API)

## Browser Compatibility

- Modern browsers with WebSocket support
- Web Crypto API for MD5 checksums (with CryptoJS fallback)
- FileReader API for reading file chunks

## Differences from HTTP Version

Unlike the HTTP-based `uppy-resumable-upload.js`, this plugin:

- Uses WebSocket instead of HTTP POST requests
- Implements binary message format for chunks
- **Uses sliding window protocol for much faster uploads** (sends 10 chunks before waiting)
- Receives asynchronous acknowledgment for each chunk
- Has built-in server-side resume protocol
- Maintains stateful connection with the server
- Supports server-initiated messages (future feature support)

## Performance

The sliding window implementation provides significant performance improvements over sequential chunk uploads:

- **10x reduction in round-trip delays**: Sends 10 chunks before waiting for ACKs instead of waiting after each chunk
- **Better network utilization**: Keeps the connection busy while server processes chunks
- **Configurable window size**: Adjust `windowSize` option based on network conditions
  - Increase for high-bandwidth, high-latency connections
  - Decrease for unreliable connections or memory-constrained clients

## Troubleshooting

### Connection Issues

- Verify WebSocket URL is correct (should start with `ws://` or `wss://`)
- Check server is running and accepting WebSocket connections
- Check browser console for connection errors

### Upload Failures

- Ensure file metadata (projectId, projectPath) is set correctly
- Verify user has permission to upload to the specified project
- Check server logs for rejection reasons
- Ensure checksum calculation is working (check for CryptoJS or Web Crypto API support)

### Resume Not Working

- Check localStorage is enabled in browser
- Verify transfer ID matches between sessions
- Check server still has transfer state (may expire after time)

## License

See project license
