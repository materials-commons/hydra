# Materials Commons Resumable Upload Plugin for Uppy

This plugin allows you to upload files to Materials Commons using the resumable upload API. It supports uploading large files by splitting them into chunks and uploading them in parallel.

## Installation

```bash
npm install @uppy/core @uppy/dashboard uppy-resumable-upload
```

## Usage

```javascript
const Uppy = require('@uppy/core')
const Dashboard = require('@uppy/dashboard')
const MaterialsCommonsResumableUpload = require('uppy-resumable-upload')

const uppy = new Uppy()
  .use(Dashboard, {
    inline: true,
    target: '#uppy-dashboard'
  })
  .use(MaterialsCommonsResumableUpload, {
    serverUrl: 'http://localhost:1352',
    apiKey: 'your-api-key',
    chunkSize: 1024 * 1024 // 1MB
  })

// Add metadata to files
uppy.on('file-added', (file) => {
  uppy.setFileMeta(file.id, {
    projectId: 123,
    destinationPath: '/path/to/destination/file.txt'
  })
})

// Start the upload
uppy.upload()
```

## Options

- `serverUrl` - The URL of the Materials Commons server. Default: `http://localhost:1352`
- `apiKey` - Your Materials Commons API key. Required.
- `chunkSize` - The size of each chunk in bytes. Default: `1024 * 1024` (1MB)
- `id` - The ID of the plugin. Default: `MaterialsCommonsResumableUpload`

## Events

The plugin emits the following events:

- `upload-started` - Emitted when the upload starts
- `upload-success` - Emitted when the upload is successful
- `upload-error` - Emitted when the upload fails

## Example

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Uppy Resumable Upload Example</title>
  <link href="https://releases.transloadit.com/uppy/v2.9.4/uppy.min.css" rel="stylesheet">
</head>
<body>
  <div id="uppy-dashboard"></div>

  <script src="https://releases.transloadit.com/uppy/v2.9.4/uppy.min.js"></script>
  <script src="uppy-resumable-upload.js"></script>
  <script>
    const uppy = new Uppy.Uppy()
      .use(Uppy.Dashboard, {
        inline: true,
        target: '#uppy-dashboard'
      })
      .use(MaterialsCommonsResumableUpload, {
        serverUrl: 'http://localhost:1352',
        apiKey: 'your-api-key',
        chunkSize: 1024 * 1024 // 1MB
      })

    // Add metadata to files
    uppy.on('file-added', (file) => {
      uppy.setFileMeta(file.id, {
        projectId: 123,
        destinationPath: '/path/to/destination/file.txt'
      })
    })

    // Start the upload
    uppy.on('dashboard:file-edit', (file) => {
      uppy.setFileMeta(file.id, {
        projectId: prompt('Enter project ID:', '123'),
        destinationPath: prompt('Enter destination path:', '/path/to/destination/' + file.name)
      })
    })
  </script>
</body>
</html>
```

## API

### `upload(fileIDs)`

Upload the specified files.

### `uploadFile(file)`

Upload a single file by splitting it into chunks and uploading each chunk.

### `uploadChunk(file, chunk, chunkIndex, totalChunks, projectId, destinationPath)`

Upload a single chunk to the server.

### `finalizeUpload(fileId, totalChunks)`

Finalize the upload by combining all chunks.

### `checkUploadStatus(fileId)`

Check the status of an upload.

## License

MIT