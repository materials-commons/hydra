/**
 * Uppy plugin for Materials Commons resumable upload API
 * Based on https://uppy.io/docs/guides/building-plugins/
 */

const { Plugin } = require('@uppy/core')
const cuid = require('cuid')

/**
 * MaterialsCommonsResumableUpload is an Uppy plugin that uploads files to
 * Materials Commons using the resumable upload API.
 */
class MaterialsCommonsResumableUpload extends Plugin {
  static VERSION = '1.0.0'

  constructor(uppy, opts) {
    super(uppy, opts)
    this.id = opts.id || 'MaterialsCommonsResumableUpload'
    this.type = 'uploader'

    // Default options
    this.opts = {
      serverUrl: 'http://localhost:1352',
      chunkSize: 1024 * 1024, // 1MB
      ...opts
    }

    // Bind methods
    this.upload = this.upload.bind(this)
    this.uploadChunk = this.uploadChunk.bind(this)
    this.finalizeUpload = this.finalizeUpload.bind(this)
    this.checkUploadStatus = this.checkUploadStatus.bind(this)
  }

  /**
   * Uppy plugin hook that is triggered when Uppy is initialized
   */
  install() {
    this.uppy.addUploader(this.upload)
  }

  /**
   * Uppy plugin hook that is triggered when the plugin is removed
   */
  uninstall() {
    this.uppy.removeUploader(this.upload)
  }

  /**
   * Main upload method that is called by Uppy when files are added
   */
  upload(fileIDs) {
    if (fileIDs.length === 0) {
      return Promise.resolve()
    }

    // Create a new Promise for each file
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
   * Upload a single file by splitting it into chunks and uploading each chunk
   */
  async uploadFile(file) {
    try {
      // Get the project ID and destination path from the file metadata
      const projectId = file.meta.projectId
      const destinationPath = file.meta.destinationPath

      if (!projectId || !destinationPath) {
        throw new Error('Project ID and destination path are required')
      }

      // Calculate the number of chunks
      const chunkSize = this.opts.chunkSize
      const totalChunks = Math.ceil(file.size / chunkSize)

      // Upload each chunk
      let fileId = null
      let fileUuid = null
      const chunkPromises = []

      for (let i = 0; i < totalChunks; i++) {
        const start = i * chunkSize
        const end = Math.min(file.size, start + chunkSize)
        const chunk = file.data.slice(start, end)

        // Create a promise for each chunk upload
        const chunkPromise = this.uploadChunk(file, chunk, i, totalChunks, projectId, destinationPath)
          .then((response) => {
            // Store the file ID and UUID from the first chunk response
            if (i === 0) {
              fileId = response.file_id
              fileUuid = response.file_uuid
            }

            // Update progress
            const uploadedChunks = this.uppy.getState().files[file.id].uploadedChunks || []
            this.uppy.setFileState(file.id, {
              uploadedChunks: [...uploadedChunks, i],
              progress: {
                uploadComplete: false,
                uploadStarted: true,
                percentage: Math.round((uploadedChunks.length + 1) / totalChunks * 100),
                bytesUploaded: (uploadedChunks.length + 1) * chunkSize,
                bytesTotal: file.size
              }
            })

            return response
          })

        chunkPromises.push(chunkPromise)
      }

      // Wait for all chunks to be uploaded
      await Promise.all(chunkPromises)

      // Finalize the upload
      const finalizeResult = await this.finalizeUpload(fileId, totalChunks)

      // Update the file state
      this.uppy.setFileState(file.id, {
        progress: {
          uploadComplete: true,
          uploadStarted: true,
          percentage: 100,
          bytesUploaded: file.size,
          bytesTotal: file.size
        }
      })

      return {
        fileId,
        fileUuid,
        ...finalizeResult
      }
    } catch (error) {
      this.uppy.log(error.stack || error.message || error)
      throw error
    }
  }

  /**
   * Upload a single chunk to the server
   */
  uploadChunk(file, chunk, chunkIndex, totalChunks, projectId, destinationPath) {
    const url = new URL(`${this.opts.serverUrl}/resumable-upload/upload`)
    url.searchParams.append('project_id', projectId)
    url.searchParams.append('destination_path', destinationPath)
    url.searchParams.append('chunk_index', chunkIndex)
    url.searchParams.append('total_chunks', totalChunks)

    // Create a FormData object to send the chunk
    const formData = new FormData()
    formData.append('file', new Blob([chunk]), file.name)

    // Get the API key from the options
    const apiKey = this.opts.apiKey

    // Upload the chunk
    return fetch(url.toString(), {
      method: 'POST',
      headers: {
        'apikey': apiKey
      },
      body: chunk,
      credentials: 'include'
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Upload failed with status ${response.status}`)
        }
        return response.json()
      })
  }

  /**
   * Finalize the upload by combining all chunks
   */
  finalizeUpload(fileId, totalChunks) {
    const url = new URL(`${this.opts.serverUrl}/resumable-upload/finalize`)
    url.searchParams.append('file_id', fileId)
    url.searchParams.append('total_chunks', totalChunks)

    // Get the API key from the options
    const apiKey = this.opts.apiKey

    // Finalize the upload
    return fetch(url.toString(), {
      method: 'POST',
      headers: {
        'apikey': apiKey
      },
      credentials: 'include'
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Finalize failed with status ${response.status}`)
        }
        return response.json()
      })
  }

  /**
   * Check the status of an upload
   */
  checkUploadStatus(fileId) {
    const url = new URL(`${this.opts.serverUrl}/resumable-upload/status`)
    url.searchParams.append('file_id', fileId)

    // Get the API key from the options
    const apiKey = this.opts.apiKey

    // Check the upload status
    return fetch(url.toString(), {
      method: 'GET',
      headers: {
        'apikey': apiKey
      },
      credentials: 'include'
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Status check failed with status ${response.status}`)
        }
        return response.json()
      })
  }
}

// Export the plugin
module.exports = MaterialsCommonsResumableUpload