Changelog
===============================================

# v3.1.2

* Removed redundant copy in `Dockerfile`
* Renamed output to `healthz` in `Dockerfile`

# v3.1.1

* Only return `err` if there is an error
* Run `httpServer` separately

# v3.1.0

* Split codec code into separate golang file
* Modified logging message for session save
* Bumped dependencies
* Build using `go` 1.20
* Updated Dockerfile for builds (+ versioning)

# v3.0.0

* RPC client is now defined once and used throughout the health check
* Use Interface object for RPC client calls
* Modified/added command line arguments

This is a significant rewrite of the health check module but should now be optimized.

# v2.0.0

* Added JSON support for https://github.com/jesec/rtorrent

# v1.0.0

Initial release