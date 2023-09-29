package mcfs

/*
Package mcfs implements a FUSE file system for the Materials Commons (MC) data repository. MC stores files
in a CAS (Content Addressable Storage) like structure. Files in MC are listed in the database in the files
table. Each file has a UUID associated with. The files are stored on disk by using a part of the UUID to
construct the path. The file is then stored using this path and named with the UUID.

The algorithm for determining the real file placement can be found in pkg/mcdb/mcmodel/file.go, the
call ToUnderlyingFilePath(mcdir string) (Which takes the root directory where files are stored in MC)
will return the underlying path.

The MCFS file system does not (currently) support file or directory removal, storing extended attributes
or soft/hard links. This implementation is used as an interface between Globus and MC. It was developed
to make it easier for users of Globus to use it with MC. This was done for a few reasons. MC supports
file versioning, checksums, and only stores one version of a file with a matching checksum. Other versions
point at the file through he uses_id field in the files table. Additionally due to the CAS structure for
file storage, Globus would present an interface that would make no sense to users. As an example of layout
if the file image.jpg had uuid 00002460-2c26-4cc1-b156-59e157b2ba18, then a user would be presented with
2c/26/00002460-2c26-4cc1-b156-59e157b2ba18 for the file.

Lastly we have an issue where we can't get the status of Globus transfers. That is Globus doesn't inform us
when a transfer is done. In the past we worked around this by requiring that users tell us when they have
completed uploads or downloads. Often users would forget to click done, and found the process confusing. The
MCFS file system now handles this form them. It tracks upload/download activity and after a period of no
activity it will automatically finish a session. Finishing a session includes things like closing out user
specific upload/download views, removing ACLs for Globus, and cleaning up other state.

MCFS presents a users project space using the project and user id, and then the directory structure for
the project. For example if project id is 1, and user id is 1, the files in the user project are stored
in directory /dir1 (directory structure can be found in the files table and is used to translate physical
layout to the logical layout), then the path is /1/1/dir1/... (files here).
*/
