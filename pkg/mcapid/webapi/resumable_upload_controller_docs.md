# Resumable Upload Controller for Unlimited, Restartable File Uploads with Chunking

This document describes how to use the Resumable Upload Controller for unlimited, restartable file uploads with support for simultaneous chunk uploads.

## Overview

The Resumable Upload Controller provides a simple API for uploading files with support for:
- Unlimited file sizes
- Multiple chunks sent simultaneously
- Tracking chunk order
- Combining chunks into a single file
- Status checking

## API Endpoints

### Upload a Chunk

```
POST /resumable-upload/upload
```

#### Query Parameters

- `project_id` (required): The ID of the project to upload the file to
- `destination_path` (required): The path where the file should be stored
- `chunk_index` (required): The index of the chunk (0-based)
- `total_chunks` (required): The total number of chunks for the file

#### Request Body

The request body should contain the chunk data to be uploaded.

#### Response

```json
{
  "file_id": 123,
  "file_uuid": "abc-123-def-456",
  "chunk_index": 0,
  "total_chunks": 5,
  "bytes_written": 1024
}
```

### Finalize Upload

```
POST /resumable-upload/finalize
```

#### Query Parameters

- `file_id` (required): The ID of the file to finalize
- `total_chunks` (required): The total number of chunks for the file

#### Response

```json
{
  "file_id": 123,
  "file_uuid": "abc-123-def-456",
  "file_size": 5120,
  "chunks": 5,
  "finalized": true
}
```

### Check Upload Status

```
GET /resumable-upload/status
```

#### Query Parameters

- `file_id` (required): The ID of the file to check

#### Response

```json
{
  "file_id": 123,
  "file_uuid": "abc-123-def-456",
  "file_size": 0,
  "exists": false,
  "has_chunks": true,
  "chunk_count": 3
}
```

## Example Usage

### Uploading Chunks

```bash
# Upload chunk 0
curl -X POST "http://localhost:1352/resumable-upload/upload?project_id=123&destination_path=/path/to/file.txt&chunk_index=0&total_chunks=3" \
  -H "apikey: your-api-key" \
  --data-binary @chunk0.txt

# Upload chunk 1
curl -X POST "http://localhost:1352/resumable-upload/upload?project_id=123&destination_path=/path/to/file.txt&chunk_index=1&total_chunks=3" \
  -H "apikey: your-api-key" \
  --data-binary @chunk1.txt

# Upload chunk 2
curl -X POST "http://localhost:1352/resumable-upload/upload?project_id=123&destination_path=/path/to/file.txt&chunk_index=2&total_chunks=3" \
  -H "apikey: your-api-key" \
  --data-binary @chunk2.txt
```

### Checking Upload Status

```bash
curl -X GET "http://localhost:1352/resumable-upload/status?file_id=123" \
  -H "apikey: your-api-key"
```

### Finalizing the Upload

```bash
curl -X POST "http://localhost:1352/resumable-upload/finalize?file_id=123&total_chunks=3" \
  -H "apikey: your-api-key"
```

### Splitting a File into Chunks (Bash Example)

```bash
# Split a file into chunks of 1MB each
split -b 1M large_file.txt chunk_

# Upload each chunk
for chunk in chunk_*; do
  index=$(echo $chunk | sed 's/chunk_//')
  curl -X POST "http://localhost:1352/resumable-upload/upload?project_id=123&destination_path=/path/to/large_file.txt&chunk_index=$index&total_chunks=$(ls chunk_* | wc -l)" \
    -H "apikey: your-api-key" \
    --data-binary @$chunk
done

# Finalize the upload
curl -X POST "http://localhost:1352/resumable-upload/finalize?file_id=123&total_chunks=$(ls chunk_* | wc -l)" \
  -H "apikey: your-api-key"
```

## Implementation Details

The Resumable Upload Controller is implemented in `pkg/mcapid/webapi/resumable_upload_controller.go` and registered in `cmd/mcapid/cmd/routes.go`.

It uses the FileStor interface to interact with the file system and database.