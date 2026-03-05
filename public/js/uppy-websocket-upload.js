/**
 * Uppy plugin for Materials Commons WebSocket-based resumable upload
 * Implements the WebSocket protocol from pkg/mctus2/wserv/client_connection.go
 *
 * Based on https://uppy.io/docs/guides/building-plugins/
 */

const { Plugin } = require('@uppy/core')
const cuid = require('cuid')

// Message type constants matching the server protocol
const MsgTransferInit = 'TRANSFER_INIT'
const MsgTransferAccept = 'TRANSFER_ACCEPT'
const MsgTransferReject = 'TRANSFER_REJECT'
const MsgTransferComplete = 'TRANSFER_COMPLETE'
const MsgTransferFinalize = 'TRANSFER_FINALIZE'
const MsgTransferResume = 'TRANSFER_RESUME'
const MsgTransferResumeResponse = 'TRANSFER_RESUME_RESPONSE'
const MsgTransferCancel = 'TRANSFER_CANCEL'
const MsgChunkAck = 'CHUNK_ACK'
const MsgUploadFailed = 'UPLOAD_FAILED'
const MsgHeartbeat = 'HEARTBEAT'

/**
 * MaterialsCommonsWebSocketUpload is an Uppy plugin that uploads files to
 * Materials Commons using the WebSocket-based resumable upload protocol.
 */
class MaterialsCommonsWebSocketUpload extends Plugin {
  static VERSION = '1.0.0'

  constructor(uppy, opts) {
    super(uppy, opts)
    this.id = opts.id || 'MaterialsCommonsWebSocketUpload'
    this.type = 'uploader'

    // Default options
    this.opts = {
      serverUrl: 'ws://localhost:1352/ws',
      chunkSize: 5 * 1024 * 1024, // 5MB default
      clientId: null, // Will be generated if not provided
      reconnectDelay: 1000,
      maxReconnectDelay: 30000,
      heartbeatInterval: 30000,
      windowSize: 10, // Number of chunks to send before waiting for ACKs
      ...opts
    }

    // Generate client ID if not provided
    if (!this.opts.clientId) {
      this.opts.clientId = cuid()
    }

    // WebSocket state
    this.ws = null
    this.connected = false
    this.reconnectAttempts = 0
    this.reconnectTimer = null
    this.heartbeatTimer = null

    // Active transfers map: transferId -> transferState
    this.activeTransfers = new Map()

    // Message handlers
    this.messageHandlers = new Map()
    this.setupMessageHandlers()

    // Bind methods
    this.upload = this.upload.bind(this)
    this.connect = this.connect.bind(this)
    this.disconnect = this.disconnect.bind(this)
    this.handleMessage = this.handleMessage.bind(this)
    this.handleBinaryMessage = this.handleBinaryMessage.bind(this)
  }

  /**
   * Setup handlers for different message types
   */
  setupMessageHandlers() {
    this.messageHandlers.set(MsgTransferAccept, this.handleTransferAccept.bind(this))
    this.messageHandlers.set(MsgTransferReject, this.handleTransferReject.bind(this))
    this.messageHandlers.set(MsgChunkAck, this.handleChunkAck.bind(this))
    this.messageHandlers.set(MsgTransferFinalize, this.handleTransferFinalize.bind(this))
    this.messageHandlers.set(MsgTransferResumeResponse, this.handleTransferResumeResponse.bind(this))
    this.messageHandlers.set(MsgUploadFailed, this.handleUploadFailed.bind(this))
    this.messageHandlers.set('HEARTBEAT_ACK', this.handleHeartbeatAck.bind(this))
  }

  /**
   * Uppy plugin hook - called when plugin is installed
   */
  install() {
    this.uppy.addUploader(this.upload)
    this.connect()
  }

  /**
   * Uppy plugin hook - called when plugin is removed
   */
  uninstall() {
    this.uppy.removeUploader(this.upload)
    this.disconnect()
  }

  /**
   * Connect to the WebSocket server
   */
  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.CONNECTING || this.ws.readyState === WebSocket.OPEN)) {
      return
    }

    this.uppy.log('[WebSocketUpload] Connecting to ' + this.opts.serverUrl)

    try {
      this.ws = new WebSocket(this.opts.serverUrl)

      this.ws.onopen = () => {
        this.uppy.log('[WebSocketUpload] Connected')
        this.connected = true
        this.reconnectAttempts = 0

        // Start heartbeat
        this.startHeartbeat()

        // Try to resume any interrupted transfers
        this.resumeInterruptedTransfers()
      }

      this.ws.onclose = (event) => {
        this.uppy.log('[WebSocketUpload] Disconnected', event.code, event.reason)
        this.connected = false
        this.stopHeartbeat()

        // Attempt reconnection
        this.scheduleReconnect()
      }

      this.ws.onerror = (error) => {
        this.uppy.log('[WebSocketUpload] Error', error)
      }

      this.ws.onmessage = (event) => {
        if (typeof event.data === 'string') {
          this.handleMessage(event.data)
        } else {
          // Binary message (shouldn't normally receive these from server in this protocol)
          this.handleBinaryMessage(event.data)
        }
      }
    } catch (error) {
      this.uppy.log('[WebSocketUpload] Connection error:', error)
      this.scheduleReconnect()
    }
  }

  /**
   * Disconnect from WebSocket server
   */
  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }

    this.stopHeartbeat()

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }

    this.connected = false
  }

  /**
   * Schedule a reconnection attempt with exponential backoff
   */
  scheduleReconnect() {
    if (this.reconnectTimer) {
      return
    }

    const delay = Math.min(
      this.opts.reconnectDelay * Math.pow(2, this.reconnectAttempts),
      this.opts.maxReconnectDelay
    )

    this.reconnectAttempts++
    this.uppy.log(`[WebSocketUpload] Reconnecting in ${delay}ms...`)

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, delay)
  }

  /**
   * Start sending heartbeat messages
   */
  startHeartbeat() {
    this.stopHeartbeat()

    this.heartbeatTimer = setInterval(() => {
      if (this.connected && this.ws.readyState === WebSocket.OPEN) {
        this.sendMessage({
          command: MsgHeartbeat,
          id: cuid(),
          clientId: this.opts.clientId,
          timestamp: new Date().toISOString()
        })
      }
    }, this.opts.heartbeatInterval)
  }

  /**
   * Stop sending heartbeat messages
   */
  stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  /**
   * Handle incoming text messages
   */
  handleMessage(data) {
    try {
      const message = JSON.parse(data)
      this.uppy.log('[WebSocketUpload] Received:', message.command, message)

      const handler = this.messageHandlers.get(message.command)
      if (handler) {
        handler(message)
      } else {
        this.uppy.log('[WebSocketUpload] Unknown message type:', message.command)
      }
    } catch (error) {
      this.uppy.log('[WebSocketUpload] Error parsing message:', error)
    }
  }

  /**
   * Handle incoming binary messages
   */
  handleBinaryMessage(data) {
    this.uppy.log('[WebSocketUpload] Received binary message (unexpected):', data.byteLength, 'bytes')
  }

  /**
   * Send a JSON message to the server
   */
  sendMessage(message) {
    if (!this.connected || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected')
    }

    message.timestamp = message.timestamp || new Date().toISOString()
    message.clientId = message.clientId || this.opts.clientId

    this.uppy.log('[WebSocketUpload] Sending:', message.command)
    this.ws.send(JSON.stringify(message))
  }

  /**
   * Send a binary chunk message
   */
  sendChunk(transferId, sequence, chunkData, isLast) {
    if (!this.connected || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected')
    }

    // Create chunk header
    const header = {
      transfer_id: transferId,
      sequence: sequence,
      size: chunkData.byteLength,
      is_last: isLast
    }

    // Convert header to JSON and add newline
    const headerStr = JSON.stringify(header) + '\n'
    const headerBytes = new TextEncoder().encode(headerStr)

    // Combine header and chunk data
    const message = new Uint8Array(headerBytes.length + chunkData.byteLength)
    message.set(headerBytes, 0)
    message.set(new Uint8Array(chunkData), headerBytes.length)

    this.uppy.log('[WebSocketUpload] Sending chunk:', sequence, chunkData.byteLength, 'bytes')
    this.ws.send(message.buffer)
  }

  /**
   * Main upload method called by Uppy
   */
  upload(fileIDs) {
    if (fileIDs.length === 0) {
      return Promise.resolve()
    }

    const promises = fileIDs.map((fileID) => {
      const file = this.uppy.getFile(fileID)
      this.uppy.emit('upload-started', file)

      return this.uploadFile(file)
        .then((result) => {
          this.uppy.emit('upload-success', file, result)
          return result
        })
        .catch((err) => {
          this.uppy.emit('upload-error', file, err)
          throw err
        })
    })

    return Promise.all(promises)
  }

  /**
   * Upload a single file
   */
  async uploadFile(file) {
    // Wait for connection
    if (!this.connected) {
      await this.waitForConnection()
    }

    // Get file metadata
    const projectId = file.meta.projectId
    const projectPath = file.meta.projectPath || file.name
    const filePath = file.meta.filePath || file.name

    if (!projectId) {
      throw new Error('Project ID is required in file metadata')
    }

    // Generate transfer ID
    const transferId = cuid()

    // Calculate checksum
    const checksum = await this.calculateChecksum(file.data)

    // Create transfer state
    const transferState = {
      transferId,
      file,
      projectId,
      projectPath,
      filePath,
      checksum,
      totalChunks: Math.ceil(file.size / this.opts.chunkSize),
      uploadedChunks: new Set(),
      sentChunks: new Set(),       // Tracks chunks sent but not yet ACKed
      currentChunk: 0,              // Next chunk to send
      startByte: 0,
      windowSize: this.opts.windowSize,
      promise: null,
      resolve: null,
      reject: null
    }

    // Create promise that will be resolved when upload completes
    transferState.promise = new Promise((resolve, reject) => {
      transferState.resolve = resolve
      transferState.reject = reject
    })

    this.activeTransfers.set(transferId, transferState)

    // Try to resume first, if there's saved state
    const savedState = this.loadTransferState(transferId)
    if (savedState && savedState.uploadedChunks > 0) {
      this.sendMessage({
        command: MsgTransferResume,
        id: transferId,
        payload: {
          transfer_id: transferId
        }
      })
    } else {
      // Start new transfer
      this.sendMessage({
        command: MsgTransferInit,
        id: transferId,
        payload: {
          transfer_id: transferId,
          project_id: projectId,
          project_path: projectPath,
          file_path: filePath,
          file_size: file.size,
          chunk_size: this.opts.chunkSize,
          checksum: checksum
        }
      })
    }

    return transferState.promise
  }

  /**
   * Handle TRANSFER_ACCEPT message
   */
  handleTransferAccept(message) {
    const { transfer_id, chunk_size } = message.payload
    const transferState = this.activeTransfers.get(transfer_id)

    if (!transferState) {
      this.uppy.log('[WebSocketUpload] Transfer not found:', transfer_id)
      return
    }

    this.uppy.log('[WebSocketUpload] Transfer accepted:', transfer_id)

    // Update chunk size if server changed it
    if (chunk_size && chunk_size !== this.opts.chunkSize) {
      transferState.totalChunks = Math.ceil(transferState.file.size / chunk_size)
    }

    // Start uploading chunks (will send up to windowSize chunks)
    this.uploadChunksInWindow(transfer_id)
  }

  /**
   * Handle TRANSFER_REJECT message
   */
  handleTransferReject(message) {
    const { transfer_id, reason } = message.payload
    const transferState = this.activeTransfers.get(transfer_id)

    if (!transferState) {
      return
    }

    this.uppy.log('[WebSocketUpload] Transfer rejected:', transfer_id, reason)
    this.activeTransfers.delete(transfer_id)
    this.removeTransferState(transfer_id)

    transferState.reject(new Error(`Transfer rejected: ${reason}`))
  }

  /**
   * Handle CHUNK_ACK message
   */
  handleChunkAck(message) {
    const { transfer_id, chunk_sequence, bytes_received } = message.payload
    const transferState = this.activeTransfers.get(transfer_id)

    if (!transferState) {
      return
    }

    // Mark chunk as uploaded and remove from sent tracking
    transferState.uploadedChunks.add(chunk_sequence)
    transferState.sentChunks.delete(chunk_sequence)

    // Update progress
    const progress = Math.round((transferState.uploadedChunks.size / transferState.totalChunks) * 100)
    this.uppy.setFileState(transferState.file.id, {
      progress: {
        uploadComplete: false,
        uploadStarted: true,
        percentage: progress,
        bytesUploaded: bytes_received,
        bytesTotal: transferState.file.size
      }
    })

    // Save state
    this.saveTransferState(transfer_id, {
      uploadedChunks: transferState.uploadedChunks.size,
      currentChunk: transferState.currentChunk
    })

    // Check if all chunks are uploaded
    if (transferState.uploadedChunks.size === transferState.totalChunks) {
      // All chunks uploaded, send complete message
      this.sendMessage({
        command: MsgTransferComplete,
        id: transfer_id,
        payload: {
          transfer_id: transfer_id
        }
      })
    } else {
      // Continue uploading - fill the window with more chunks
      this.uploadChunksInWindow(transfer_id)
    }
  }

  /**
   * Handle TRANSFER_FINALIZE message
   */
  handleTransferFinalize(message) {
    const { transfer_id, status } = message.payload
    const transferState = this.activeTransfers.get(transfer_id)

    if (!transferState) {
      return
    }

    this.uppy.log('[WebSocketUpload] Transfer finalized:', transfer_id, status)

    // Update final progress
    this.uppy.setFileState(transferState.file.id, {
      progress: {
        uploadComplete: true,
        uploadStarted: true,
        percentage: 100,
        bytesUploaded: transferState.file.size,
        bytesTotal: transferState.file.size
      }
    })

    // Clean up
    this.activeTransfers.delete(transfer_id)
    this.removeTransferState(transfer_id)

    transferState.resolve({
      transferId: transfer_id,
      status: status
    })
  }

  /**
   * Handle TRANSFER_RESUME_RESPONSE message
   */
  handleTransferResumeResponse(message) {
    const { transfer_id, can_resume, resume_from_chunk, bytes_received } = message.payload
    const transferState = this.activeTransfers.get(transfer_id)

    if (!transferState) {
      return
    }

    if (can_resume) {
      this.uppy.log('[WebSocketUpload] Resuming transfer from chunk:', resume_from_chunk)

      // Update state to resume from the right position
      transferState.currentChunk = resume_from_chunk
      transferState.startByte = bytes_received

      // Mark previous chunks as uploaded
      for (let i = 0; i < resume_from_chunk; i++) {
        transferState.uploadedChunks.add(i)
      }

      // Continue uploading - fill the window
      this.uploadChunksInWindow(transfer_id)
    } else {
      // Can't resume, start fresh
      this.uppy.log('[WebSocketUpload] Cannot resume, starting fresh')
      transferState.reject(new Error('Cannot resume transfer'))
      this.activeTransfers.delete(transfer_id)
    }
  }

  /**
   * Handle UPLOAD_FAILED message
   */
  handleUploadFailed(message) {
    const { transfer_id, error } = message.payload
    const transferState = this.activeTransfers.get(transfer_id)

    if (!transferState) {
      return
    }

    this.uppy.log('[WebSocketUpload] Upload failed:', transfer_id, error)
    this.activeTransfers.delete(transfer_id)
    this.removeTransferState(transfer_id)

    transferState.reject(new Error(`Upload failed: ${error}`))
  }

  /**
   * Handle HEARTBEAT_ACK message
   */
  handleHeartbeatAck(message) {
    // Nothing to do, just confirms connection is alive
  }

  /**
   * Upload chunks to fill the sliding window
   * Sends multiple chunks up to windowSize without waiting for individual ACKs
   */
  async uploadChunksInWindow(transferId) {
    const transferState = this.activeTransfers.get(transferId)
    if (!transferState) {
      return
    }

    // Calculate how many chunks we can send
    // Window = chunks sent but not yet ACKed
    const windowUsed = transferState.sentChunks.size
    const windowAvailable = transferState.windowSize - windowUsed

    if (windowAvailable <= 0) {
      // Window is full, wait for ACKs
      return
    }

    // Send chunks to fill the window
    const chunksToSend = []
    for (let i = 0; i < windowAvailable; i++) {
      const chunkIndex = transferState.currentChunk + i

      // Don't send beyond total chunks or chunks already uploaded
      if (chunkIndex >= transferState.totalChunks) {
        break
      }

      if (transferState.uploadedChunks.has(chunkIndex)) {
        continue
      }

      chunksToSend.push(chunkIndex)
    }

    // Send all chunks in parallel
    const sendPromises = chunksToSend.map(chunkIndex =>
      this.sendSingleChunk(transferId, chunkIndex)
    )

    await Promise.all(sendPromises)
  }

  /**
   * Send a single chunk
   */
  async sendSingleChunk(transferId, chunkIndex) {
    const transferState = this.activeTransfers.get(transferId)
    if (!transferState) {
      return
    }

    const { file, totalChunks } = transferState
    const chunkSize = this.opts.chunkSize
    const start = chunkIndex * chunkSize
    const end = Math.min(file.size, start + chunkSize)
    const isLast = chunkIndex === totalChunks - 1

    // Read chunk data
    const chunkData = await this.readChunk(file.data, start, end)

    // Mark as sent (before actual send to track in-flight chunks)
    transferState.sentChunks.add(chunkIndex)

    // Update currentChunk to track the next chunk to send
    if (chunkIndex >= transferState.currentChunk) {
      transferState.currentChunk = chunkIndex + 1
    }

    // Send chunk
    this.sendChunk(transferId, chunkIndex, chunkData, isLast)
  }

  /**
   * Read a chunk from the file
   */
  readChunk(fileData, start, end) {
    return new Promise((resolve, reject) => {
      if (fileData instanceof Blob) {
        const slice = fileData.slice(start, end)
        const reader = new FileReader()
        reader.onload = () => resolve(reader.result)
        reader.onerror = reject
        reader.readAsArrayBuffer(slice)
      } else {
        // Already an ArrayBuffer
        resolve(fileData.slice(start, end))
      }
    })
  }

  /**
   * Calculate MD5 checksum of file
   */
  async calculateChecksum(fileData) {
    // Use the Web Crypto API if available
    if (window.crypto && window.crypto.subtle) {
      try {
        let buffer
        if (fileData instanceof Blob) {
          buffer = await fileData.arrayBuffer()
        } else {
          buffer = fileData
        }

        const hashBuffer = await window.crypto.subtle.digest('MD5', buffer)
        const hashArray = Array.from(new Uint8Array(hashBuffer))
        return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
      } catch (error) {
        this.uppy.log('[WebSocketUpload] Crypto API MD5 not available, using fallback')
      }
    }

    // Fallback: if crypto-js is available
    if (typeof CryptoJS !== 'undefined') {
      const wordArray = CryptoJS.lib.WordArray.create(fileData)
      return CryptoJS.MD5(wordArray).toString()
    }

    // If neither is available, return empty string (server may reject)
    this.uppy.log('[WebSocketUpload] Warning: No MD5 implementation available')
    return ''
  }

  /**
   * Wait for WebSocket connection
   */
  waitForConnection(timeout = 10000) {
    return new Promise((resolve, reject) => {
      if (this.connected) {
        resolve()
        return
      }

      const startTime = Date.now()
      const checkInterval = setInterval(() => {
        if (this.connected) {
          clearInterval(checkInterval)
          resolve()
        } else if (Date.now() - startTime > timeout) {
          clearInterval(checkInterval)
          reject(new Error('Connection timeout'))
        }
      }, 100)
    })
  }

  /**
   * Resume interrupted transfers from previous session
   */
  resumeInterruptedTransfers() {
    // Could load from localStorage and attempt to resume
    // For now, just log
    this.uppy.log('[WebSocketUpload] Checking for interrupted transfers...')
  }

  /**
   * Cancel an active transfer
   */
  cancelTransfer(transferId) {
    const transferState = this.activeTransfers.get(transferId)
    if (!transferState) {
      return
    }

    this.sendMessage({
      command: MsgTransferCancel,
      id: transferId,
      payload: {
        transfer_id: transferId
      }
    })

    this.activeTransfers.delete(transferId)
    this.removeTransferState(transferId)

    transferState.reject(new Error('Transfer cancelled by user'))
  }

  /**
   * Save transfer state to localStorage
   */
  saveTransferState(transferId, state) {
    try {
      const key = `mc_transfer_${transferId}`
      localStorage.setItem(key, JSON.stringify(state))
    } catch (error) {
      this.uppy.log('[WebSocketUpload] Error saving transfer state:', error)
    }
  }

  /**
   * Load transfer state from localStorage
   */
  loadTransferState(transferId) {
    try {
      const key = `mc_transfer_${transferId}`
      const data = localStorage.getItem(key)
      return data ? JSON.parse(data) : null
    } catch (error) {
      this.uppy.log('[WebSocketUpload] Error loading transfer state:', error)
      return null
    }
  }

  /**
   * Remove transfer state from localStorage
   */
  removeTransferState(transferId) {
    try {
      const key = `mc_transfer_${transferId}`
      localStorage.removeItem(key)
    } catch (error) {
      this.uppy.log('[WebSocketUpload] Error removing transfer state:', error)
    }
  }
}

// Export the plugin
module.exports = MaterialsCommonsWebSocketUpload
