# bespoke

Package bespoke provides a way to create custom binaries: files that are executable
that also contain additional data. The data can either be a key-value map or an
arbitrary file.

## Motivation

A common use for the Go language is creating command-line tools. As a concrete example,
consider a web application that allows its users to download a command-line
client to interact with it. The client may need to be configured with such
things as the user name or an access token. Using bespoke you can create a binary
that is specifically configured for each user who downloads it.

See the [documentation](http://godoc.org/github.com/vikasgorur/bespoke) for more information.